package rd08v1196

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/devices/xiaomi/common/routerweb"
	"github.com/sunerpy/router-tools/internal/secret"
)

const tcpProbeTimeout = 5 * time.Second

// PortProbeFunc probes a TCP port without sending application data.
type PortProbeFunc func(ctx context.Context, host string, port int) device.PortState

// Prober implements the RD08 read-only doctor probe.
type Prober struct {
	probePort     PortProbeFunc
	sshDial       SSHDialFunc
	telnetDial    TelnetDialFunc
	webLogin      WebLoginFunc
	macGenerator  func() ([3]string, error)
	sleep         func(time.Duration)
	downRounds    int
	onlineTimeout time.Duration
}

// NewProber constructs a prober. A nil port probe uses the production TCP implementation.
func NewProber(probePort PortProbeFunc) *Prober {
	if probePort == nil {
		probePort = probeTCPPort
	}
	return &Prober{
		probePort:    probePort,
		sshDial:      dialSSH,
		telnetDial:   dialTelnet,
		webLogin:     loginWeb,
		macGenerator: generateMACs,
		sleep:        time.Sleep,
	}
}

// Probe checks Web TCP, fetches identity over GET, then observes SSH and Telnet ports.
func (p *Prober) Probe(ctx context.Context, target device.Target) (device.ProbeResult, error) {
	if err := target.Validate(); err != nil {
		return device.ProbeResult{}, err
	}
	state := p.probePort(ctx, target.Host, target.WebPort)
	if ctx.Err() != nil {
		return device.ProbeResult{}, device.WrapError(device.CodeCanceled, "operation canceled", ctx.Err())
	}
	if state != device.PortOpen {
		return device.ProbeResult{}, &device.Error{
			Code:      device.CodeWebUnreachable,
			Message:   fmt.Sprintf("Web TCP port is %s", state),
			Retryable: true,
		}
	}
	baseURL, err := target.WebBaseURL()
	if err != nil {
		return device.ProbeResult{}, err
	}
	client, err := routerweb.NewClient(baseURL)
	if err != nil {
		return device.ProbeResult{}, &device.Error{Code: device.CodeInvalidArgument, Message: err.Error()}
	}
	observation, err := client.Probe(ctx)
	if err != nil {
		return device.ProbeResult{}, normalizeWebError(err)
	}
	return device.ProbeResult{
		Observation: observation,
		SSH:         p.probePort(ctx, target.Host, target.SSHPort),
		Telnet:      p.probePort(ctx, target.Host, target.TelnetPort),
	}, nil
}

// VerifyAdmin performs Web login and immediately drops the opaque session token.
func (p *Prober) VerifyAdmin(ctx context.Context, target device.Target, password secret.Password) error {
	if err := target.Validate(); err != nil {
		return err
	}
	if err := password.Validate(); err != nil {
		return err
	}
	baseURL, err := target.WebBaseURL()
	if err != nil {
		return err
	}
	client, err := routerweb.NewClient(baseURL)
	if err != nil {
		return &device.Error{Code: device.CodeInvalidArgument, Message: err.Error()}
	}
	session, err := client.Login(ctx, password)
	if err != nil {
		return normalizeWebError(err)
	}
	session.Close()
	return nil
}

func normalizeWebError(err error) error {
	if errors.Is(err, context.Canceled) {
		return device.WrapError(device.CodeCanceled, "operation canceled", context.Canceled)
	}
	var deviceErr *device.Error
	if errors.As(err, &deviceErr) {
		return err
	}
	code := device.CodeWebProtocolError
	var networkError net.Error
	if errors.As(err, &networkError) || errors.Is(err, context.DeadlineExceeded) {
		code = device.CodeWebUnreachable
	}
	return &device.Error{Code: code, Message: err.Error()}
}

func probeTCPPort(ctx context.Context, host string, port int) device.PortState {
	probeCtx, cancel := context.WithTimeout(ctx, tcpProbeTimeout)
	defer cancel()
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	connection, err := (&net.Dialer{Timeout: tcpProbeTimeout}).DialContext(probeCtx, "tcp", address)
	if err != nil {
		var networkError net.Error
		if errors.As(err, &networkError) && networkError.Timeout() {
			return device.PortUnreachable
		}
		return device.PortClosed
	}
	_ = connection.Close()
	return device.PortOpen
}
