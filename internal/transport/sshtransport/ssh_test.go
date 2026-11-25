package sshtransport_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/transport/shell"
	"github.com/sunerpy/router-tools/internal/transport/sshtransport"
)

type fixture struct {
	stdout []byte
	stderr []byte
	exit   uint32
	delay  time.Duration
}

type sshServer struct {
	listener net.Listener
	config   *ssh.ServerConfig
	mu       sync.Mutex
	fixtures map[string]fixture
}

func TestSSHRunCommandsAndLargeStderr(t *testing.T) {
	t.Parallel()

	largeStderr := bytes.Repeat([]byte("E"), 3<<20)
	server := startSSHServer(t, "root-password", map[string]fixture{
		"id -u":   {stdout: []byte("0\n"), exit: 0},
		"large":   {stdout: []byte("stdout-done"), stderr: largeStderr, exit: 0},
		"nonzero": {stdout: []byte("partial-out"), stderr: []byte("partial-err"), exit: 7},
	})
	password, _ := secret.NewPassword("root-password")
	client, err := sshtransport.Dial(t.Context(), sshtransport.Config{
		Host: server.host(), Port: server.port(), Password: password,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error { return nil },
	})
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer func() { _ = client.Close() }()

	result, err := client.Run(t.Context(), "large", shell.RunOptions{})
	if err != nil {
		t.Fatalf("Run(large) error = %v", err)
	}
	if string(result.Stdout) != "stdout-done" || !bytes.Equal(result.Stderr, largeStderr) {
		t.Fatalf("large result sizes = stdout:%d stderr:%d", len(result.Stdout), len(result.Stderr))
	}
	root, err := shell.VerifyRoot(t.Context(), client)
	if err != nil || !root {
		t.Fatalf("VerifyRoot() = %t, %v", root, err)
	}
	result, err = client.Run(t.Context(), "nonzero", shell.RunOptions{})
	var remoteErr *shell.RemoteCommandError
	if !errors.As(err, &remoteErr) {
		t.Fatalf("Run(nonzero) error = %T %v", err, err)
	}
	if result.ExitCode != 7 || string(result.Stdout) != "partial-out" || string(result.Stderr) != "partial-err" {
		t.Fatalf("nonzero result = %+v", result)
	}
}

func TestSSHStreamsBinaryWithoutBufferingWholePartition(t *testing.T) {
	t.Parallel()

	binaryData := []byte{0, 255, 1, 2, 0, 255}
	server := startSSHServer(t, "password", map[string]fixture{
		"cat /dev/mtd0": {stdout: binaryData, stderr: []byte("diagnostic"), exit: 0},
	})
	password, _ := secret.NewPassword("password")
	client, err := sshtransport.Dial(t.Context(), sshtransport.Config{
		Host: server.host(), Port: server.port(), Password: password,
		HostKeyCallback: func(string, net.Addr, ssh.PublicKey) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	var destination bytes.Buffer
	result, err := client.Stream(t.Context(), "cat /dev/mtd0", &destination, int64(len(binaryData)))
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	if !bytes.Equal(destination.Bytes(), binaryData) || string(result.Stderr) != "diagnostic" || !result.Started {
		t.Fatalf("Stream() = bytes:%v result:%+v", destination.Bytes(), result)
	}
}

func TestSSHRejectsWrongPassword(t *testing.T) {
	t.Parallel()

	server := startSSHServer(t, "correct", nil)
	password, _ := secret.NewPassword("wrong")
	_, err := sshtransport.Dial(t.Context(), sshtransport.Config{
		Host: server.host(), Port: server.port(), Password: password,
		HostKeyCallback: func(string, net.Addr, ssh.PublicKey) error { return nil },
	})
	assertDeviceCode(t, err, device.CodeSSHAuthFailed)
}

func TestSSHTimeoutClosesConnection(t *testing.T) {
	t.Parallel()

	server := startSSHServer(t, "password", map[string]fixture{
		"slow": {stdout: []byte("late"), delay: 200 * time.Millisecond},
	})
	password, _ := secret.NewPassword("password")
	client, err := sshtransport.Dial(t.Context(), sshtransport.Config{
		Host: server.host(), Port: server.port(), Password: password,
		HostKeyCallback: func(string, net.Addr, ssh.PublicKey) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Run(t.Context(), "slow", shell.RunOptions{Timeout: 30 * time.Millisecond})
	if err == nil || !result.Started {
		t.Fatalf("Run(slow) = %+v, %v", result, err)
	}
	if _, err := client.Run(t.Context(), "id -u", shell.RunOptions{}); err == nil {
		t.Fatal("client remained reusable after timeout")
	}
}

func TestSSHRequiresHostKeyCallbackBeforeNetworkAccess(t *testing.T) {
	t.Parallel()

	password, _ := secret.NewPassword("password")
	_, err := sshtransport.Dial(t.Context(), sshtransport.Config{Host: "192.0.2.1", Port: 22, Password: password})
	if err == nil || !strings.Contains(err.Error(), "host key") {
		t.Fatalf("Dial() error = %v", err)
	}
}

func startSSHServer(t *testing.T, password string, fixtures map[string]fixture) *sshServer {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{PasswordCallback: func(metadata ssh.ConnMetadata, given []byte) (*ssh.Permissions, error) {
		if metadata.User() == "root" && string(given) == password {
			return nil, nil
		}
		return nil, fmt.Errorf("authentication failed")
	}}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &sshServer{listener: listener, config: config, fixtures: fixtures}
	t.Cleanup(func() { _ = listener.Close() })
	go server.serve()
	return server
}

func (s *sshServer) host() string { return s.listener.Addr().(*net.TCPAddr).IP.String() }
func (s *sshServer) port() int    { return s.listener.Addr().(*net.TCPAddr).Port }

func (s *sshServer) serve() {
	for {
		connection, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(connection)
	}
}

func (s *sshServer) handle(connection net.Conn) {
	defer func() { _ = connection.Close() }()
	serverConnection, channels, requests, err := ssh.NewServerConn(connection, s.config)
	if err != nil {
		return
	}
	defer func() { _ = serverConnection.Close() }()
	go ssh.DiscardRequests(requests)
	for channelRequest := range channels {
		if channelRequest.ChannelType() != "session" {
			_ = channelRequest.Reject(ssh.UnknownChannelType, "session only")
			continue
		}
		channel, channelRequests, err := channelRequest.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer func() { _ = channel.Close() }()
			for request := range channelRequests {
				if request.Type != "exec" {
					_ = request.Reply(false, nil)
					continue
				}
				_ = request.Reply(true, nil)
				commandLength := binary.BigEndian.Uint32(request.Payload[:4])
				s.mu.Lock()
				fixture := s.fixtures[string(request.Payload[4:4+commandLength])]
				s.mu.Unlock()
				if fixture.delay > 0 {
					time.Sleep(fixture.delay)
				}
				writeChunks(channel, fixture.stdout)
				writeChunks(channel.Stderr(), fixture.stderr)
				_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{fixture.exit}))
				return
			}
		}()
	}
}

func writeChunks(writer interface{ Write([]byte) (int, error) }, data []byte) {
	for len(data) > 0 {
		size := min(len(data), 997)
		_, _ = writer.Write(data[:size])
		data = data[size:]
	}
}

func assertDeviceCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %s", want)
	}
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) || deviceErr.Code != want {
		t.Fatalf("error = %T %v, want %s", err, err, want)
	}
}
