package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/guiapi"
	"github.com/sunerpy/router-tools/internal/mcpserver"
)

const (
	// mcpShutdownWait bounds how long routerctl mcp waits for canceled operations at exit.
	mcpShutdownWait  = 20 * time.Second
	maxTokenFileSize = 4096
)

type mcpFlags struct {
	httpAddress string
	tokenFile   string
	allowWrites bool
}

func addMCPCommands(root *cobra.Command, options *operationOptions) {
	flags := &mcpFlags{}
	command := &cobra.Command{
		Use:   "mcp",
		Short: "Serve the agent tools over the Model Context Protocol (stdio unless --http)",
		Long: `Serve the Router Tools agent tools over the Model Context Protocol.

Without --http the server speaks newline-delimited JSON on stdin and stdout; an agent host starts it as a
subprocess. With --http it serves Streamable HTTP at http://<address>/mcp and every request needs
"Authorization: Bearer <token>" (the token comes from --token-file or ROUTERCTL_MCP_TOKEN).

Router writes (guide_apply, soft_remove) are offered only with --allow-writes. Deep persistence, recovery
acknowledgement, forgetting saved passwords and Telnet are never offered. See RISKS.md before using it.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runMCP(cmd.Context(), options, *flags, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	command.Flags().StringVar(&flags.httpAddress, "http", "", "serve Streamable HTTP at host:port (an IP address or localhost) instead of stdio")
	command.Flags().BoolVar(&flags.allowWrites, "allow-writes", false, "also offer the router-write tools guide_apply and soft_remove")
	command.Flags().StringVar(&flags.tokenFile, "token-file", "", "file with the HTTP access token (default: $"+mcpserver.TokenEnv+")")
	command.AddCommand(&cobra.Command{
		Use:   "token",
		Short: "Print a new random access token for routerctl mcp --http",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			token, err := mcpserver.NewToken()
			if err != nil {
				return &ExitError{Code: 12, Err: errors.New("cannot create a random token")}
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), token)
			return err
		},
	})
	root.AddCommand(command)
}

func runMCP(ctx context.Context, options *operationOptions, flags mcpFlags, stdout, stderr io.Writer) error {
	application, ok := options.service.(guiapi.Application)
	if !ok {
		return &ExitError{Code: 12, Err: errors.New("routerctl mcp is not configured in this build")}
	}
	verifier := mcpserver.NewTokenVerifier()
	var listen mcpserver.Listen
	if flags.httpAddress != "" {
		parsed, err := mcpserver.ParseListen(flags.httpAddress)
		if err != nil {
			return &ExitError{Code: 2, Err: err}
		}
		listen = parsed
		token, err := readMCPToken(flags.tokenFile)
		if err != nil {
			return &ExitError{Code: 2, Err: err}
		}
		if err := verifier.SetToken(token); err != nil {
			return &ExitError{Code: 2, Err: err}
		}
	} else if flags.tokenFile != "" {
		return &ExitError{Code: 2, Err: &device.Error{Code: device.CodeInvalidArgument, Message: "--token-file is only used with --http"}}
	}

	operations := mcpserver.NewOperations()
	// No host-key confirmer: without a pin, an existing known_hosts entry, or accept_new_host_key with a
	// given root password, an unknown host key is refused.
	frontend := guiapi.New(application, operations, nil)
	server := mcpserver.New(frontend, operations, mcpserver.Options{
		Version: options.info.Version, AllowWrites: flags.allowWrites, Audit: mcpserver.NewJSONAudit(stderr),
	})
	if flags.allowWrites {
		_, _ = fmt.Fprintln(stderr, "routerctl mcp: router writes are enabled for agents (guide_apply, soft_remove)")
	}

	var serveErr error
	if flags.httpAddress == "" {
		serveErr = server.ServeIO(ctx, options.stdin, stdout)
	} else {
		listener, err := net.Listen("tcp", listen.Address)
		if err != nil {
			return &ExitError{Code: 12, Err: fmt.Errorf("listen on %s: %w", listen.Address, err)}
		}
		if !listen.Loopback {
			_, _ = fmt.Fprintln(stderr, "routerctl mcp: WARNING: this address is reachable from the network; the access token and any passwords in tool arguments travel in plaintext")
		}
		_, _ = fmt.Fprintf(stderr, "routerctl mcp: listening on http://%s%s\n", listener.Addr().String(), mcpserver.EndpointPath)
		// Stop first when the process is told to exit, so requests waiting for an operation end at once and
		// the HTTP server can close within its grace period.
		served := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				server.Stop()
			case <-served:
			}
		}()
		serveErr = mcpserver.Serve(ctx, listener, server.Handler(verifier))
		close(served)
	}

	shutdown, cancel := context.WithTimeout(context.Background(), mcpShutdownWait)
	defer cancel()
	if still := server.Shutdown(shutdown); len(still) > 0 {
		_, _ = fmt.Fprintf(stderr, "routerctl mcp: operations still running at exit: %s\n", strings.Join(still, ", "))
		return &ExitError{Code: 10, Err: errors.New("operations were still running at exit; a router write left this way blocks further writes until routerctl recovery ack")}
	}
	if serveErr != nil && !errors.Is(serveErr, context.Canceled) && !errors.Is(serveErr, io.EOF) && !errors.Is(serveErr, io.ErrClosedPipe) {
		return &ExitError{Code: 12, Err: fmt.Errorf("serve MCP: %w", serveErr)}
	}
	return nil
}

// readMCPToken reads the HTTP access token from a private file or the environment, never echoing it.
func readMCPToken(path string) (string, error) {
	if path == "" {
		token := strings.TrimSpace(os.Getenv(mcpserver.TokenEnv))
		if token == "" {
			return "", &device.Error{
				Code:    device.CodeInvalidArgument,
				Message: "--http needs an access token: set " + mcpserver.TokenEnv + " or pass --token-file (create one with routerctl mcp token)",
			}
		}
		return token, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", &device.Error{Code: device.CodeInvalidArgument, Message: "cannot read the token file"}
	}
	if !info.Mode().IsRegular() || info.Size() > maxTokenFileSize {
		return "", &device.Error{Code: device.CodeInvalidArgument, Message: "the token file must be a small regular file"}
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return "", &device.Error{Code: device.CodeInvalidArgument, Message: "the token file must not be readable by group or others (chmod 600)"}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", &device.Error{Code: device.CodeInvalidArgument, Message: "cannot read the token file"}
	}
	return strings.TrimSpace(string(data)), nil
}
