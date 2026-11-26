package mcpserver

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/guiapi"
)

const (
	toolListProfiles    = "list_profiles"
	toolPreflight       = "preflight"
	toolGuidePlan       = "guide_plan"
	toolDeepStatus      = "deep_status"
	toolBackupCreate    = "backup_create"
	toolSavedPasswords  = "saved_passwords"
	toolOperationWait   = "operation_wait"
	toolOperationList   = "operation_list"
	toolOperationCancel = "operation_cancel"
	toolGuideApply      = "guide_apply"
	toolSoftRemove      = "soft_remove"

	defaultWait    = 20 * time.Second
	maxWaitSeconds = 50
)

// toolClass is the side-effect class a tool's annotations advertise.
type toolClass int

const (
	// classR has no side effect at all.
	classR toolClass = iota
	// classL leaves the router untouched but may change the local password vault or files.
	classL
	// classW writes the router or stops an operation that may be writing it.
	classW
)

func annotations(class toolClass, title string) *mcp.ToolAnnotations {
	no, yes := false, true
	switch class {
	case classR:
		return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, DestructiveHint: &no, OpenWorldHint: &no}
	case classL:
		return &mcp.ToolAnnotations{Title: title, DestructiveHint: &no, OpenWorldHint: &yes}
	default:
		return &mcp.ToolAnnotations{Title: title, DestructiveHint: &yes, OpenWorldHint: &yes}
	}
}

type toolSpec struct {
	name        string
	title       string
	description string
	class       toolClass
	schema      *jsonschema.Schema
	handle      func(context.Context, *mcp.CallToolRequest, time.Time) *mcp.CallToolResult
}

// addTool registers a tool with the low-level SDK API: the SDK then does no argument validation (whose
// messages can quote argument values); handlers decode strictly themselves.
func (s *Server) addTool(spec toolSpec) {
	s.mcp.AddTool(&mcp.Tool{
		Name: spec.name, Title: spec.title, Description: spec.description,
		InputSchema: spec.schema, Annotations: annotations(spec.class, spec.title),
	}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		started := s.now()
		if s.closed.Load() {
			return s.reject(started, spec.name, &device.Error{Code: device.CodeCanceled, Message: "the MCP server is shutting down"}), nil
		}
		return spec.handle(ctx, request, started), nil
	})
}

func schemaFor[T any](writeOnly ...string) *jsonschema.Schema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		panic(fmt.Sprintf("mcpserver: schema for %T: %v", *new(T), err))
	}
	for _, name := range writeOnly {
		if property, ok := schema.Properties[name]; ok {
			property.WriteOnly = true
		}
	}
	return schema
}

// Tool arguments. Password fields carry no schema constraints and are marked write-only.

type targetArgs struct {
	Host    string `json:"host" jsonschema:"router IP address or host name, without a scheme or path"`
	WebPort int    `json:"web_port,omitempty" jsonschema:"router Web (HTTP) port; default 80"`
	SSHPort int    `json:"ssh_port,omitempty" jsonschema:"router SSH port; default 22"`
}

type hostKeyArgs struct {
	HostKeySHA256    string `json:"host_key_sha256,omitempty" jsonschema:"expected SSH host key fingerprint (SHA256:...); only a fingerprint the user gave you; not allowed with use_saved_password"`
	AcceptNewHostKey bool   `json:"accept_new_host_key,omitempty" jsonschema:"store an SSH host key seen for the first time; only with a given root_password and only after the user agreed"`
}

type credentialArgs struct {
	UseSavedPassword bool `json:"use_saved_password,omitempty" jsonschema:"fill passwords that are not given from the encrypted vault"`
	RememberPassword bool `json:"remember_password,omitempty" jsonschema:"save the given passwords in the encrypted vault once the router accepted them; only when the user asks"`
}

type waitArgs struct {
	WaitSeconds *int `json:"wait_seconds,omitempty" jsonschema:"seconds to wait for the result before returning done=false (0 to 50, default 20); then continue with operation_wait"`
}

type routerArgs struct {
	Target    targetArgs `json:"target" jsonschema:"the router"`
	ProfileID string     `json:"profile_id,omitempty" jsonschema:"exact profile ID from list_profiles; default auto"`
	hostKeyArgs
	credentialArgs
	waitArgs
}

type preflightArgs struct {
	routerArgs
	CheckAdminLogin bool   `json:"check_admin_login,omitempty" jsonschema:"also verify the Web admin login (needs admin_password or use_saved_password)"`
	CheckRootLogin  bool   `json:"check_root_login,omitempty" jsonschema:"also verify root login over SSH (needs root_password or use_saved_password)"`
	AdminPassword   string `json:"admin_password,omitempty" jsonschema:"router Web admin password; never repeat it back"`
	RootPassword    string `json:"root_password,omitempty" jsonschema:"router root password; never repeat it back"`
}

type guidePlanArgs struct {
	routerArgs
	Goals             []string `json:"goals,omitempty" jsonschema:"subset of ssh, backup, soft, reboot; default ssh, backup, soft (ssh is always included; reboot needs soft)"`
	ResetRootPassword bool     `json:"reset_root_password,omitempty" jsonschema:"when SSH is open but the root password is unknown, plan to set a new one; only when the user asks"`
	RootPassword      string   `json:"root_password,omitempty" jsonschema:"current root password, used only for read-only SSH detection; never repeat it back"`
}

type rootArgs struct {
	routerArgs
	RootPassword string `json:"root_password,omitempty" jsonschema:"router root password; never repeat it back"`
}

type softRemoveArgs struct {
	routerArgs
	RootPassword string `json:"root_password,omitempty" jsonschema:"router root password; never repeat it back"`
	Confirm      bool   `json:"confirm" jsonschema:"must be true, set only after the user approved removing soft persistence"`
}

type guideApplyArgs struct {
	PlanID        string `json:"plan_id" jsonschema:"plan_id returned by guide_plan"`
	PlanDigest    string `json:"plan_digest" jsonschema:"report.plan_digest returned by guide_plan"`
	AdminPassword string `json:"admin_password,omitempty" jsonschema:"router Web admin password, when the plan's required_inputs lists admin_password; never repeat it back"`
	RootPassword  string `json:"root_password,omitempty" jsonschema:"the new root password when required_inputs lists new_root_password, otherwise the current one; never repeat it back"`
	hostKeyArgs
	credentialArgs
	waitArgs
}

type operationWaitArgs struct {
	OperationID string `json:"operation_id" jsonschema:"operation_id returned by an earlier call"`
	waitArgs
}

type operationCancelArgs struct {
	OperationID string `json:"operation_id" jsonschema:"operation_id of a running operation this server started"`
}

type noArgs struct{}

func (s *Server) readTools() []toolSpec {
	return []toolSpec{
		{
			name: toolListProfiles, title: "List supported routers", class: classR, schema: schemaFor[noArgs](),
			description: "List the supported router models and their exact firmware profiles. Does not contact any router.",
			handle:      s.handleListProfiles,
		},
		{
			name: toolPreflight, title: "Preflight check (does not change the router)", class: classL,
			schema:      schemaFor[preflightArgs]("admin_password", "root_password"),
			description: "Probe a router read-only: identify the model, firmware and open ports, and optionally verify the Web admin login and the SSH root login. Does not change the router; with remember_password or use_saved_password it may update the local password vault.",
			handle:      s.handlePreflight,
		},
		{
			name: toolGuidePlan, title: "Plan guided setup (does not change the router)", class: classL,
			schema:      schemaFor[guidePlanArgs]("root_password"),
			description: "Detect the router's state read-only and plan only the missing steps of the guided setup (enable SSH, backup, soft persistence, optional reboot check). Returns the steps with their effects, required_inputs, and, when ready, plan_id and report.plan_digest for guide_apply. Show the steps to the user and get approval before guide_apply.",
			handle:      s.handleGuidePlan,
		},
		{
			name: toolDeepStatus, title: "Deep persistence status (does not change the router)", class: classL,
			schema:      schemaFor[rootArgs]("root_password"),
			description: "Read the deep-persistence readiness of a router: policy, managed journal, verified backups and, with a root password, the device flags. Read-only; deep persistence itself is human-only.",
			handle:      s.handleDeepStatus,
		},
		{
			name: toolBackupCreate, title: "Back up all partitions (does not change the router)", class: classL,
			schema:      schemaFor[rootArgs]("root_password"),
			description: "Read every MTD partition over SSH into a new managed backup directory on this computer and verify it. Only reads the router. Takes minutes: continue with operation_wait.",
			handle:      s.handleBackupCreate,
		},
		{
			name: toolSavedPasswords, title: "List saved passwords", class: classR, schema: schemaFor[noArgs](),
			description: "List which router passwords are saved in the encrypted vault (kind, model, host, port, rejected or not). Never returns password values.",
			handle:      s.handleSavedPasswords,
		},
		{
			name: toolOperationWait, title: "Wait for an operation", class: classR, schema: schemaFor[operationWaitArgs](),
			description: "Wait up to wait_seconds for an operation this server started and return its progress or its final report.",
			handle:      s.handleOperationWait,
		},
		{
			name: toolOperationList, title: "List operations", class: classR, schema: schemaFor[noArgs](),
			description: "List the operations this server started, newest first, with their status.",
			handle:      s.handleOperationList,
		},
		{
			name: toolOperationCancel, title: "Cancel an operation", class: classW, schema: schemaFor[operationCancelArgs](),
			description: "Cancel a running operation this server started. Canceling a write after it began can leave the router partly changed; the final report says so. Only when the user asks.",
			handle:      s.handleOperationCancel,
		},
	}
}

func (s *Server) writeTools() []toolSpec {
	return []toolSpec{
		{
			name: toolGuideApply, title: "Apply the guided plan (changes the router)", class: classW,
			schema:      schemaFor[guideApplyArgs]("admin_password", "root_password"),
			description: "Run a plan from guide_plan once, after the user approved its steps and effects. It detects again and stops before any write if the router changed. Each plan_id works once; never call it again after a timeout or a partial or unknown result.",
			handle:      s.handleGuideApply,
		},
		{
			name: toolSoftRemove, title: "Remove soft persistence (changes the router)", class: classW,
			schema:      schemaFor[softRemoveArgs]("root_password"),
			description: "Remove the managed soft SSH persistence (boot script and firewall include). Requires confirm=true after the user approved it. Does not restore the old root password or disable SSH.",
			handle:      s.handleSoftRemove,
		},
	}
}

// Handlers.

func (s *Server) handleListProfiles(_ context.Context, request *mcp.CallToolRequest, started time.Time) *mcp.CallToolResult {
	var args noArgs
	if err := decodeArguments(request, &args); err != nil {
		return s.reject(started, toolListProfiles, err)
	}
	return s.respond(started, reply{called: toolListProfiles, result: toolResult{
		Tool: toolListProfiles, Done: true, Status: app.StatusSucceeded, Report: s.backend.Catalog(),
	}})
}

func (s *Server) handleSavedPasswords(_ context.Context, request *mcp.CallToolRequest, started time.Time) *mcp.CallToolResult {
	var args noArgs
	if err := decodeArguments(request, &args); err != nil {
		return s.reject(started, toolSavedPasswords, err)
	}
	report, err := s.backend.CredentialStatus()
	result := reportOutcome(report, report.Status, report.Error, err)
	return s.respond(started, reply{
		called: toolSavedPasswords, failure: isFailure(result.status, result.err), errorCode: result.errorCode(),
		result: toolResult{Tool: toolSavedPasswords, Done: true, Status: result.status, Report: result.report, Error: result.err},
	})
}

func (s *Server) handlePreflight(ctx context.Context, request *mcp.CallToolRequest, started time.Time) *mcp.CallToolResult {
	var args preflightArgs
	if err := decodeArguments(request, &args); err != nil {
		return s.reject(started, toolPreflight, err)
	}
	target, profile, wait, err := args.resolve()
	if err != nil {
		return s.reject(started, toolPreflight, err)
	}
	dto := s.operationRequest(target, profile, args.routerArgs)
	dto.AdminPassword, dto.RootPassword = args.AdminPassword, args.RootPassword
	dto.WithAdminAuth, dto.WithRootVerify = args.CheckAdminLogin, args.CheckRootLogin
	return s.runOperation(ctx, request, started, toolPreflight, target.Host, wait, newRedactor(args.AdminPassword, args.RootPassword),
		func(ctx context.Context, id string) outcome {
			dto.OperationID = id
			report, err := s.backend.Doctor(ctx, dto)
			return reportOutcome(report, report.Status, report.Error, err)
		})
}

func (s *Server) handleGuidePlan(ctx context.Context, request *mcp.CallToolRequest, started time.Time) *mcp.CallToolResult {
	var args guidePlanArgs
	if err := decodeArguments(request, &args); err != nil {
		return s.reject(started, toolGuidePlan, err)
	}
	target, profile, wait, err := args.resolve()
	if err != nil {
		return s.reject(started, toolGuidePlan, err)
	}
	goals := args.Goals
	if len(goals) == 0 {
		goals = []string{app.GoalSSH, app.GoalBackup, app.GoalSoft}
	}
	for _, goal := range goals {
		switch goal {
		case app.GoalSSH, app.GoalBackup, app.GoalSoft, app.GoalReboot:
		default:
			// The value is not echoed: an agent could have put anything there.
			return s.reject(started, toolGuidePlan, invalid("goals must be a subset of ssh, backup, soft and reboot"))
		}
	}
	dto := guiapi.GuideRequest{
		Target: target, ProfileID: profile, Goals: goals, RootPassword: args.RootPassword,
		ResetRootPassword: args.ResetRootPassword, ToolVersion: s.version, HostKey: args.hostKey(),
		UseSavedPassword: args.UseSavedPassword, RememberPassword: args.RememberPassword,
	}
	return s.runOperation(ctx, request, started, toolGuidePlan, target.Host, wait, newRedactor(args.RootPassword),
		func(ctx context.Context, id string) outcome {
			dto.OperationID = id
			report, err := s.backend.GuidePlan(ctx, dto)
			result := reportOutcome(report, report.Status, report.Error, err)
			if err == nil && report.Ready && report.Confirmation != nil && report.PlanDigest != "" {
				planID, storeErr := s.plans.put(*report.Confirmation, report.PlanDigest)
				if storeErr != nil {
					result.err = &device.Error{Code: device.CodeInternalError, Message: "cannot keep the plan for guide_apply"}
				} else {
					result.planID = planID
				}
			}
			return result
		})
}

func (s *Server) handleDeepStatus(ctx context.Context, request *mcp.CallToolRequest, started time.Time) *mcp.CallToolResult {
	var args rootArgs
	if err := decodeArguments(request, &args); err != nil {
		return s.reject(started, toolDeepStatus, err)
	}
	target, profile, wait, err := args.resolve()
	if err != nil {
		return s.reject(started, toolDeepStatus, err)
	}
	dto := s.operationRequest(target, profile, args.routerArgs)
	dto.RootPassword = args.RootPassword
	return s.runOperation(ctx, request, started, toolDeepStatus, target.Host, wait, newRedactor(args.RootPassword),
		func(ctx context.Context, id string) outcome {
			dto.OperationID = id
			report, err := s.backend.DeepStatus(ctx, dto)
			return reportOutcome(report, report.Status, report.Error, err)
		})
}

func (s *Server) handleBackupCreate(ctx context.Context, request *mcp.CallToolRequest, started time.Time) *mcp.CallToolResult {
	var args rootArgs
	if err := decodeArguments(request, &args); err != nil {
		return s.reject(started, toolBackupCreate, err)
	}
	target, profile, wait, err := args.resolve()
	if err != nil {
		return s.reject(started, toolBackupCreate, err)
	}
	dto := s.operationRequest(target, profile, args.routerArgs)
	dto.RootPassword = args.RootPassword
	return s.runOperation(ctx, request, started, toolBackupCreate, target.Host, wait, newRedactor(args.RootPassword),
		func(ctx context.Context, id string) outcome {
			dto.OperationID = id
			report, err := s.backend.BackupCreate(ctx, dto)
			return reportOutcome(report, report.Status, report.Error, err)
		})
}

func (s *Server) handleGuideApply(ctx context.Context, request *mcp.CallToolRequest, started time.Time) *mcp.CallToolResult {
	var args guideApplyArgs
	if err := decodeArguments(request, &args); err != nil {
		return s.reject(started, toolGuideApply, err)
	}
	if args.PlanID == "" || args.PlanDigest == "" {
		return s.reject(started, toolGuideApply, invalid("plan_id and plan_digest from guide_plan are required"))
	}
	if err := checkPin(args.hostKeyArgs, args.credentialArgs); err != nil {
		return s.reject(started, toolGuideApply, err)
	}
	wait, err := args.wait()
	if err != nil {
		return s.reject(started, toolGuideApply, err)
	}
	hostKey := args.hostKey()
	return s.runWrite(ctx, request, started, toolGuideApply, s.plans.host(args.PlanID), wait, newRedactor(args.AdminPassword, args.RootPassword),
		func(ctx context.Context, id string) outcome {
			// Taking the plan inside the operation keeps it when no operation slot was free.
			confirmation, digest, takeErr := s.plans.take(args.PlanID, args.PlanDigest)
			if takeErr != nil {
				return outcome{status: app.StatusFailed, err: takeErr}
			}
			report, err := s.backend.GuideApply(ctx, guiapi.GuideRequest{
				OperationID: id, Confirmation: &confirmation, PlanDigest: digest, Confirmed: true,
				AdminPassword: args.AdminPassword, RootPassword: args.RootPassword, ToolVersion: s.version, HostKey: hostKey,
				UseSavedPassword: args.UseSavedPassword, RememberPassword: args.RememberPassword,
			})
			return reportOutcome(report, report.Status, report.Error, err)
		})
}

func (s *Server) handleSoftRemove(ctx context.Context, request *mcp.CallToolRequest, started time.Time) *mcp.CallToolResult {
	var args softRemoveArgs
	if err := decodeArguments(request, &args); err != nil {
		return s.reject(started, toolSoftRemove, err)
	}
	target, profile, wait, err := args.resolve()
	if err != nil {
		return s.reject(started, toolSoftRemove, err)
	}
	if !args.Confirm {
		return s.reject(started, toolSoftRemove, &device.Error{
			Code: device.CodeConfirmationRequired, Message: "confirm must be true; get the user's approval to remove soft persistence first",
		})
	}
	dto := s.operationRequest(target, profile, args.routerArgs)
	dto.RootPassword, dto.Confirmed = args.RootPassword, true
	return s.runWrite(ctx, request, started, toolSoftRemove, device.CanonicalHost(target.Host), wait, newRedactor(args.RootPassword),
		func(ctx context.Context, id string) outcome {
			dto.OperationID = id
			report, err := s.backend.PersistRemove(ctx, dto)
			return reportOutcome(report, report.Status, report.Error, err)
		})
}

func (s *Server) handleOperationWait(ctx context.Context, request *mcp.CallToolRequest, started time.Time) *mcp.CallToolResult {
	var args operationWaitArgs
	if err := decodeArguments(request, &args); err != nil {
		return s.reject(started, toolOperationWait, err)
	}
	wait, err := args.wait()
	if err != nil {
		return s.reject(started, toolOperationWait, err)
	}
	op, ok := s.ops.get(args.OperationID)
	if !ok {
		return s.reject(started, toolOperationWait, unknownOperation())
	}
	return s.awaitAndRespond(ctx, request, started, toolOperationWait, op, wait)
}

func (s *Server) handleOperationList(_ context.Context, request *mcp.CallToolRequest, started time.Time) *mcp.CallToolResult {
	var args noArgs
	if err := decodeArguments(request, &args); err != nil {
		return s.reject(started, toolOperationList, err)
	}
	return s.respond(started, reply{called: toolOperationList, result: toolResult{
		Tool: toolOperationList, Done: true, Status: app.StatusSucceeded, Report: s.ops.List(),
	}})
}

func (s *Server) handleOperationCancel(_ context.Context, request *mcp.CallToolRequest, started time.Time) *mcp.CallToolResult {
	var args operationCancelArgs
	if err := decodeArguments(request, &args); err != nil {
		return s.reject(started, toolOperationCancel, err)
	}
	op, ok := s.ops.get(args.OperationID)
	if !ok {
		return s.reject(started, toolOperationCancel, unknownOperation())
	}
	requested := false
	select {
	case <-op.done:
	default:
		op.cancel()
		requested = true
	}
	return s.respond(started, reply{called: toolOperationCancel, host: op.host, result: toolResult{
		OperationID: op.id, Tool: toolOperationCancel, Done: true, Status: app.StatusSucceeded,
		Report: map[string]bool{"cancel_requested": requested},
	}})
}

// Request building and validation.

func (a routerArgs) resolve() (device.Target, string, time.Duration, *device.Error) {
	if err := checkPin(a.hostKeyArgs, a.credentialArgs); err != nil {
		return device.Target{}, "", 0, err
	}
	if a.Target.Host == "" {
		return device.Target{}, "", 0, invalid("target.host is required")
	}
	target := device.Target{Host: a.Target.Host, WebPort: 80, SSHPort: 22, TelnetPort: 23}
	if a.Target.WebPort != 0 {
		target.WebPort = a.Target.WebPort
	}
	if a.Target.SSHPort != 0 {
		target.SSHPort = a.Target.SSHPort
	}
	if err := target.Validate(); err != nil {
		return device.Target{}, "", 0, toDeviceError(err)
	}
	profile := a.ProfileID
	if profile == "" {
		profile = "auto"
	}
	wait, err := a.wait()
	if err != nil {
		return device.Target{}, "", 0, err
	}
	return target, profile, wait, nil
}

// checkPin refuses an agent-supplied fingerprint pin together with saved passwords. A pin replaces the
// known_hosts check, so it would let a saved root password reach a host key the user never accepted
// (spec §16.7(4), §23.5).
func checkPin(hostKey hostKeyArgs, credentials credentialArgs) *device.Error {
	if hostKey.HostKeySHA256 != "" && credentials.UseSavedPassword {
		return invalid("host_key_sha256 cannot be combined with use_saved_password: saved passwords are only sent to host keys already in known_hosts; pass the password instead, and pin only a fingerprint the user gave you")
	}
	return nil
}

func (w waitArgs) wait() (time.Duration, *device.Error) {
	if w.WaitSeconds == nil {
		return defaultWait, nil
	}
	if *w.WaitSeconds < 0 || *w.WaitSeconds > maxWaitSeconds {
		return 0, invalid("wait_seconds must be between 0 and 50")
	}
	return time.Duration(*w.WaitSeconds) * time.Second, nil
}

func (h hostKeyArgs) hostKey() guiapi.HostKeyRequest {
	return guiapi.HostKeyRequest{PinSHA256: h.HostKeySHA256, AcceptNew: h.AcceptNewHostKey}
}

// operationRequest fixes everything an agent must not choose: SSH only, no Telnet, the default known_hosts,
// the managed directories, and this server's tool version.
func (s *Server) operationRequest(target device.Target, profile string, args routerArgs) guiapi.OperationRequest {
	return guiapi.OperationRequest{
		Target: target, ProfileID: profile, Transport: "ssh", AllowTelnet: false, ToolVersion: s.version,
		HostKey: args.hostKey(), UseSavedPassword: args.UseSavedPassword, RememberPassword: args.RememberPassword,
	}
}

var argumentPointer = regexp.MustCompile(`^(/[a-z][a-z0-9_]{0,39}){1,4}$`)

// decodeArguments decodes exactly: member names must match case-sensitively, unknown and duplicate names are
// refused (encoding/json/v2), and error messages never contain a name or value the client sent.
func decodeArguments(request *mcp.CallToolRequest, target any) *device.Error {
	raw := []byte("{}")
	if request != nil && request.Params != nil && len(request.Params.Arguments) > 0 {
		raw = request.Params.Arguments
	}
	if string(bytes.TrimSpace(raw)) == "null" {
		raw = []byte("{}")
	}
	if err := jsonv2.Unmarshal(raw, target, jsonv2.RejectUnknownMembers(true)); err != nil {
		return argumentError(err)
	}
	return nil
}

func argumentError(err error) *device.Error {
	if errors.Is(err, jsontext.ErrDuplicateName) {
		return invalid("an argument is given more than once")
	}
	if errors.Is(err, jsonv2.ErrUnknownName) {
		return invalid("unknown argument; the tool's input schema lists the accepted names")
	}
	var semantic *jsonv2.SemanticError
	if errors.As(err, &semantic) {
		pointer := string(semantic.JSONPointer)
		if argumentPointer.MatchString(pointer) && semantic.GoType != nil {
			return invalid(fmt.Sprintf("%s must be %s", strings.ReplaceAll(strings.TrimPrefix(pointer, "/"), "/", "."), jsonKind(semantic.GoType)))
		}
		return invalid("the arguments do not match this tool's schema")
	}
	return invalid("the arguments must be one JSON object matching this tool's schema")
}

func jsonKind(t reflect.Type) string {
	if t == nil {
		return "of another type"
	}
	switch t.Kind() {
	case reflect.Bool:
		return "a boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "an integer"
	case reflect.String:
		return "a string"
	case reflect.Slice, reflect.Array:
		return "an array"
	case reflect.Pointer:
		return jsonKind(t.Elem())
	default:
		return "an object"
	}
}

func invalid(message string) *device.Error {
	return &device.Error{Code: device.CodeInvalidArgument, Message: message}
}

func unknownOperation() *device.Error {
	return invalid("operation_id is not an operation of this server (finished operations are kept until 50 newer ones finish)")
}

func toDeviceError(err error) *device.Error {
	var deviceErr *device.Error
	if errors.As(err, &deviceErr) {
		copied := *deviceErr
		return &copied
	}
	return &device.Error{Code: device.CodeInternalError, Message: err.Error()}
}

// bindingError normalizes an error the frontend returned besides (or instead of) a report.
func bindingError(err error) *device.Error {
	if errors.Is(err, context.Canceled) {
		return &device.Error{Code: device.CodeCanceled, Message: "operation canceled"}
	}
	return toDeviceError(err)
}

// reportOutcome turns a frontend result into an outcome. A report without a status (the call failed before
// the use case ran) is dropped and the status becomes failed.
func reportOutcome(report any, status string, reportErr *device.Error, err error) outcome {
	result := outcome{status: status, report: report}
	if reportErr != nil {
		result.reportCode = reportErr.Code
	}
	if err != nil {
		result.err = bindingError(err)
	}
	if status == "" || status == statusRunning {
		result.status, result.report = app.StatusFailed, nil
	}
	return result
}

func isFailure(status string, err *device.Error) bool {
	switch {
	case err != nil:
		return true
	case status == app.StatusFailed, status == app.StatusPartial, status == app.StatusUnknown:
		return true
	default:
		return false
	}
}
