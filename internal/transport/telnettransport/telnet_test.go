package telnettransport_test

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/transport/shell"
	"github.com/sunerpy/router-tools/internal/transport/telnettransport"
)

func TestTelnetLoginAndCommandFrames(t *testing.T) {
	t.Parallel()

	address := startTelnetServer(t, "password", "0", 0, false)
	password, _ := secret.NewPassword("password")
	client, err := telnettransport.Dial(t.Context(), telnettransport.Config{Address: address, Password: password})
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer func() { _ = client.Close() }()
	root, err := shell.VerifyRoot(t.Context(), client)
	if err != nil || !root {
		t.Fatalf("VerifyRoot() = %t, %v", root, err)
	}
}

func TestTelnetRejectsWrongPassword(t *testing.T) {
	t.Parallel()

	address := startTelnetServer(t, "password", "", 0, true)
	password, _ := secret.NewPassword("wrong")
	_, err := telnettransport.Dial(t.Context(), telnettransport.Config{Address: address, Password: password})
	if err == nil {
		t.Fatal("Dial() error = nil")
	}
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) || deviceErr.Code != device.CodeTelnetAuthFailed {
		t.Fatalf("Dial() error = %T %v", err, err)
	}
}

func TestTelnetNonZeroAndTimeout(t *testing.T) {
	t.Parallel()

	t.Run("non-zero", func(t *testing.T) {
		address := startTelnetServer(t, "password", "failure", 7, false)
		password, _ := secret.NewPassword("password")
		client, err := telnettransport.Dial(t.Context(), telnettransport.Config{Address: address, Password: password})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = client.Close() }()
		result, err := client.Run(t.Context(), "false", shell.RunOptions{})
		var remoteErr *shell.RemoteCommandError
		if !errors.As(err, &remoteErr) || result.ExitCode != 7 || strings.TrimSpace(string(result.Stdout)) != "failure" {
			t.Fatalf("Run() = %+v, %T %v", result, err, err)
		}
	})

	t.Run("timeout closes", func(t *testing.T) {
		address := startTelnetServer(t, "password", "late", 0, false)
		password, _ := secret.NewPassword("password")
		client, err := telnettransport.Dial(t.Context(), telnettransport.Config{Address: address, Password: password})
		if err != nil {
			t.Fatal(err)
		}
		result, err := client.Run(t.Context(), "slow", shell.RunOptions{Timeout: 30 * time.Millisecond})
		if err == nil || !result.Started {
			t.Fatalf("Run(slow) = %+v, %v", result, err)
		}
		if _, err := client.Run(t.Context(), "id -u", shell.RunOptions{}); err == nil {
			t.Fatal("client reusable after timeout")
		}
	})
}

func startTelnetServer(t *testing.T, password, output string, exitCode int, reject bool) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = connection.Close() }()
		for _, value := range []byte{255, 251, 1, 255, 253, 3} {
			_, _ = connection.Write([]byte{value})
		}
		_, _ = connection.Write([]byte("LoGiN: "))
		reader := bufio.NewReader(connection)
		_, _ = reader.ReadString('\n')
		_, _ = connection.Write([]byte("PASSWORD: "))
		line, _ := reader.ReadString('\n')
		if reject || strings.TrimSpace(line) != password {
			_, _ = connection.Write([]byte("login incorrect\r\n"))
			return
		}
		_, _ = connection.Write([]byte("# "))
		command, _ := reader.ReadString('\n')
		begin := regexp.MustCompile(`__BEGIN_[0-9a-f]+__`).FindString(command)
		end := regexp.MustCompile(`__END_[0-9a-f]+__`).FindString(command)
		if strings.Contains(command, "'slow'") {
			time.Sleep(200 * time.Millisecond)
			return
		}
		response := command + "\r\n" + begin + "\r\n" + output + "\r\n" + end + ":" + fmt.Sprint(exitCode) + "\r\n# "
		for _, value := range []byte(response) {
			_, _ = connection.Write([]byte{value})
		}
	}()
	return listener.Addr().String()
}
