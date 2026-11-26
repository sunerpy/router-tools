// Package agentaccess is the desktop's agent access: the MCP server over the desktop's guiapi.Service, its
// Streamable HTTP listener with stored settings, the recent agent activity, per-host setup snippets and skill
// installation. The desktop bindings are thin wrappers around a Controller.
package agentaccess

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/sunerpy/router-tools/internal/agentsetup"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/mcpserver"
	"github.com/sunerpy/router-tools/skills"
)

// activityRecords is how many audit records the desktop keeps.
const activityRecords = 200

// Options configure a Controller.
type Options struct {
	// SettingsDir holds settings.json (<UserConfigDir>/routerctl/mcp).
	SettingsDir string
	Version     string
	// GOOS selects snippet quoting and paths.
	GOOS string
	// Home and Getenv resolve the hosts' skill and configuration directories.
	Home   func() (string, error)
	Getenv func(string) string
	// Local lists this computer's addresses; nil uses the network interfaces.
	Local func() ([]netip.Addr, error)
	// OnChange receives the status after a change of the listener, an operation or the activity. Calls are
	// coalesced and made from one goroutine, never while the controller holds a lock.
	OnChange func(Status)
}

// Status is everything the desktop page shows.
type Status struct {
	Listener mcpserver.ListenerStatus `json:"listener"`
	// URL is the endpoint local agents use: the listen address, or loopback for a wildcard.
	URL            string                       `json:"url"`
	LocalAddresses []string                     `json:"local_addresses"`
	Operations     []mcpserver.OperationSummary `json:"operations"`
	Activity       []mcpserver.AuditRecord      `json:"activity"`
	Agents         []agentsetup.Agent           `json:"agents"`
	// SkillPaths is where InstallSkill puts the skill for each host with a skills directory, after the hosts'
	// override variables; a host whose directory cannot be resolved is missing.
	SkillPaths map[string]string `json:"skill_paths"`
}

// SkillRequest installs the skill for one host (Agent) or into a directory the user picked (Dir).
type SkillRequest struct {
	Agent string `json:"agent,omitempty"`
	Dir   string `json:"dir,omitempty"`
	Force bool   `json:"force,omitempty"`
}

// SkillReport is the outcome of an installation: installed, unchanged or updated; differs when an installed copy
// differs and Force was not set; refused for a symbolic link or non-directory in the way; failed otherwise.
type SkillReport struct {
	Status  string   `json:"status"`
	Path    string   `json:"path"`
	Files   []string `json:"files,omitempty"`
	Message string   `json:"message,omitempty"`
}

// Controller owns the desktop's MCP server and listener.
type Controller struct {
	options  Options
	server   *mcpserver.Server
	ops      *mcpserver.Operations
	listener *mcpserver.Listener
	ring     *mcpserver.AuditRing

	changes chan struct{}
	stop    chan struct{}
	once    sync.Once
}

// New builds the server over backend; ops must be the registry that receives the backend's progress events
// (pass it to guiapi.New as part of the emitter).
func New(backend mcpserver.Backend, ops *mcpserver.Operations, options Options) *Controller {
	if options.Getenv == nil {
		options.Getenv = func(string) string { return "" }
	}
	if options.Home == nil {
		options.Home = os.UserHomeDir
	}
	c := &Controller{options: options, ops: ops, ring: mcpserver.NewAuditRing(activityRecords), changes: make(chan struct{}, 1), stop: make(chan struct{})}
	audit := mcpserver.MultiAudit(c.ring, mcpserver.AuditFunc(func(mcpserver.AuditRecord) { c.signal() }))
	c.server = mcpserver.New(backend, ops, mcpserver.Options{Version: options.Version, Audit: audit})
	c.listener = mcpserver.NewListener(c.server, mcpserver.ListenerOptions{
		Store: mcpserver.SettingsStore{Dir: options.SettingsDir}, Local: options.Local,
		OnChange: func(mcpserver.ListenerStatus) { c.signal() }, Describe: describe,
	})
	ops.OnChange(c.signal)
	return c
}

// Start applies the stored settings and starts delivering change notifications.
func (c *Controller) Start() Status {
	go c.deliver()
	c.listener.Start()
	return c.Status()
}

func (c *Controller) signal() {
	select {
	case c.changes <- struct{}{}:
	default:
	}
}

func (c *Controller) deliver() {
	for {
		select {
		case <-c.stop:
			return
		case <-c.changes:
			if c.options.OnChange != nil {
				c.options.OnChange(c.Status())
			}
		}
	}
}

// Status reports the listener, the endpoint, the operations and the recent activity.
func (c *Controller) Status() Status {
	listener := c.listener.Status()
	return Status{
		Listener: listener, URL: endpoint(listener), LocalAddresses: mcpserver.LocalAddresses(),
		Operations: c.ops.List(), Activity: c.ring.Records(), Agents: agentsetup.Agents(), SkillPaths: c.skillPaths(),
	}
}

// skillPaths resolves each host's skill target the way InstallSkill does.
func (c *Controller) skillPaths() map[string]string {
	paths := map[string]string{}
	home, err := c.options.Home()
	if err != nil {
		return paths
	}
	for _, agent := range agentsetup.Agents() {
		if agent.SkillDir == "" {
			continue
		}
		if root, err := agentsetup.SkillRoot(agent.ID, home, c.options.Getenv); err == nil {
			paths[agent.ID] = filepath.Join(root, skills.Name)
		}
	}
	return paths
}

// endpoint is the URL a local agent uses: the configured host, or loopback for a wildcard.
func endpoint(status mcpserver.ListenerStatus) string {
	host := status.ListenHost
	if addr, err := netip.ParseAddr(host); err == nil && addr.IsUnspecified() {
		host = "127.0.0.1"
		if addr.Is6() {
			host = "::1"
		}
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(status.Port)) + mcpserver.EndpointPath
}

// Apply changes the listener and the write switch (see mcpserver.Listener.Apply). A failure reads as the
// status's last error does.
func (c *Controller) Apply(change mcpserver.SettingsChange) (mcpserver.ApplyResult, error) {
	result, err := c.listener.Apply(change)
	if err != nil {
		return result, described{err}
	}
	return result, nil
}

// described is an error in the words the desktop shows; errors.Is and errors.As still see the cause.
type described struct{ err error }

func (d described) Error() string { return describe(d.err) }
func (d described) Unwrap() error { return d.err }

// describe explains a port another program holds, the failure the operator can fix, and keeps the system's
// text (which names the address) for everything else and as the detail.
func describe(err error) string {
	if mcpserver.AddressInUse(err) {
		return "端口已被其他程序占用：换一个端口，或先关闭占用它的程序（" + err.Error() + "）"
	}
	return err.Error()
}

// NewToken replaces the access token and returns it once.
func (c *Controller) NewToken() (string, error) {
	return c.listener.NewToken()
}

// Setup is the Streamable HTTP setup of one host for the current endpoint, with Chinese notes; the token stays
// in the variable ROUTERCTL_MCP_TOKEN. WorkBuddy documents only stdio servers and gets an error.
func (c *Controller) Setup(agent string) (agentsetup.Setup, error) {
	return agentsetup.Build(agentsetup.Request{
		Agent: agent, URL: endpoint(c.listener.Status()), OS: c.options.GOOS, Getenv: c.options.Getenv, Lang: "zh",
	})
}

// InstallSkill installs the embedded skill for a host with a documented skills directory, or into an existing
// absolute directory the user picked. Invalid requests are errors; installation outcomes are reports.
func (c *Controller) InstallSkill(request SkillRequest) (SkillReport, error) {
	if (request.Agent == "") == (request.Dir == "") {
		return SkillReport{}, invalid("choose a host or a directory")
	}
	root := request.Dir
	if request.Agent != "" {
		home, err := c.options.Home()
		if err != nil {
			return SkillReport{}, fmt.Errorf("locate the home directory: %w", err)
		}
		root, err = agentsetup.SkillRoot(request.Agent, home, c.options.Getenv)
		if err != nil {
			return SkillReport{}, invalid(err.Error())
		}
	} else {
		if !filepath.IsAbs(root) {
			return SkillReport{}, invalid("the skills directory must be an absolute path")
		}
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			return SkillReport{}, invalid("the skills directory must be an existing directory")
		}
	}
	files, err := skills.Files()
	if err != nil {
		return SkillReport{}, err
	}
	target := filepath.Join(root, skills.Name)
	result, err := agentsetup.InstallSkill(skills.RouterTools(), files, root, skills.Name, request.Force)
	switch {
	case err == nil:
		return SkillReport{Status: result.Action, Path: result.Path, Files: result.Files}, nil
	case errors.Is(err, agentsetup.ErrDiffers):
		return SkillReport{Status: "differs", Path: target, Message: err.Error()}, nil
	case errors.Is(err, agentsetup.ErrRefused):
		return SkillReport{Status: "refused", Path: target, Message: err.Error()}, nil
	default:
		return SkillReport{Status: "failed", Path: target, Message: err.Error()}, nil
	}
}

func invalid(message string) error {
	return &device.Error{Code: device.CodeInvalidArgument, Message: message}
}

// Owns reports whether an operation ID belongs to an agent operation.
func (c *Controller) Owns(operationID string) bool {
	return c.ops.Has(operationID)
}

// Cancel requests the cancellation of a running agent operation.
func (c *Controller) Cancel(operationID string) bool {
	return c.ops.Cancel(operationID)
}

// Shutdown stops listening, cancels the running agent operations and waits for them until ctx ends; it returns
// the IDs that did not stop. It stops the change notifications.
func (c *Controller) Shutdown(ctx context.Context) []string {
	c.listener.Close()
	remaining := c.server.Shutdown(ctx)
	c.once.Do(func() { close(c.stop) })
	return remaining
}
