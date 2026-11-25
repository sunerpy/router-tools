package rd08v1196

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/devices/xiaomi/common/routerweb"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

// WebSession is the restricted MAC-rule channel required by this adapter.
type WebSession interface {
	AddMACRule(context.Context, string, string) error
	DeleteMACRule(context.Context, string) error
	Close()
}

// WebLoginFunc authenticates a new Web session.
type WebLoginFunc func(context.Context, device.Target, secret.Password) (WebSession, error)

func loginWeb(ctx context.Context, target device.Target, password secret.Password) (WebSession, error) {
	baseURL, err := target.WebBaseURL()
	if err != nil {
		return nil, err
	}
	client, err := routerweb.NewClient(baseURL)
	if err != nil {
		return nil, err
	}
	return client.Login(ctx, password)
}

func generateMACs() ([3]string, error) {
	const rangeSize = 0xD0 - 0x10 + 1
	value, err := rand.Int(rand.Reader, big.NewInt(rangeSize))
	if err != nil {
		return [3]string{}, fmt.Errorf("generate MAC base: %w", err)
	}
	base := int(value.Int64()) + 0x10
	return [3]string{
		fmt.Sprintf("0A:11:22:33:44:%02X", base),
		fmt.Sprintf("0A:11:22:33:44:%02X", base+1),
		fmt.Sprintf("0A:11:22:33:44:%02X", base+2),
	}, nil
}

// EnableSSH implements the three fixed RD08 commands and mandatory cleanup.
func (p *Prober) EnableSSH(ctx context.Context, target device.Target, adminPassword, rootPassword secret.Password, policy hostkey.Policy) (app.EnableResult, error) {
	result := app.EnableResult{Status: app.StatusFailed}
	redactor := secret.NewRedactor(adminPassword.Expose(), rootPassword.Expose())
	session, err := p.webLogin(ctx, target, adminPassword)
	if err != nil {
		return result, normalizeWebError(err)
	}
	defer session.Close()
	if verification, verifyErr := p.verifySSH(ctx, target, rootPassword, policy); verifyErr == nil && verification.RootVerified {
		result.Status = app.StatusSucceeded
		result.Outcome = app.OutcomeAlreadyVerified
		result.SSHVerified = true
		return result, nil
	} else if verifyErr != nil {
		var deviceErr *device.Error
		if errors.As(verifyErr, &deviceErr) && (deviceErr.Code == device.CodeHostKeyMismatch || deviceErr.Code == device.CodeSSHHostKeyRejected) {
			return result, verifyErr
		}
		result.Warnings = append(result.Warnings, "existing SSH did not accept the new root credentials; continuing with the fixed enable flow")
	}

	macs, err := p.macGenerator()
	if err != nil {
		return result, err
	}
	plain := rootPassword.Expose() + "\n" + rootPassword.Expose() + "\n"
	encodedPassword := base64.StdEncoding.EncodeToString([]byte(plain))
	commands := []string{
		"nvram set ssh_en=1 && nvram set telnet_en=1 && nvram commit",
		`sed -i "s/channel=.*/channel=\"debug\"/g" /etc/init.d/dropbear && /etc/init.d/dropbear start`,
		"printf '%s' '" + encodedPassword + "' | base64 -d | passwd root",
	}
	var primaryErr error
	for index, command := range commands {
		mac := macs[index]
		result.AttemptedMACs = append(result.AttemptedMACs, mac)
		if err := session.AddMACRule(ctx, mac, command); err != nil {
			primaryErr = err
			break
		}
		result.AcceptedSteps++
	}
	if result.AcceptedSteps == len(commands) {
		p.sleep(2 * time.Second)
	}

	cleanupFailed := false
	for _, mac := range result.AttemptedMACs {
		cleanup := app.CleanupResult{MAC: mac}
		for attempt := 1; attempt <= 2; attempt++ {
			cleanup.Attempts = attempt
			err := session.DeleteMACRule(ctx, mac)
			if err == nil {
				cleanup.Success = true
				cleanup.Error = ""
				break
			}
			cleanup.Error = redactor.Text(err.Error())
			var deviceErr *device.Error
			if !errors.As(err, &deviceErr) || deviceErr.Code != device.CodeRemoteResultUnknown {
				break
			}
		}
		if !cleanup.Success {
			cleanupFailed = true
		}
		result.Cleanup = append(result.Cleanup, cleanup)
	}
	if cleanupFailed {
		result.Warnings = append(result.Warnings, "one or more temporary MAC rules could not be removed; inspect the router Web UI")
	}
	if primaryErr != nil {
		var deviceErr *device.Error
		if errors.As(primaryErr, &deviceErr) && deviceErr.Code == device.CodeRemoteResultUnknown {
			result.Status = app.StatusUnknown
		} else if result.AcceptedSteps > 0 {
			result.Status = app.StatusPartial
		}
		return result, primaryErr
	}

	var verifyErr error
	for attempt := 0; attempt < 3; attempt++ {
		verification, err := p.verifySSH(ctx, target, rootPassword, policy)
		if err == nil && verification.RootVerified {
			result.SSHVerified = true
			verifyErr = nil
			break
		}
		verifyErr = err
		if attempt < 2 {
			p.sleep(2 * time.Second)
		}
	}
	if !result.SSHVerified {
		result.Status = app.StatusPartial
		if verifyErr == nil {
			verifyErr = &device.Error{Code: device.CodeRootVerifyFailed, Message: "SSH root verification failed after three attempts"}
		}
		return result, verifyErr
	}
	if cleanupFailed {
		result.Status = app.StatusPartial
		return result, &device.Error{Code: device.CodeMACCleanupFailed, Message: "SSH is verified but temporary MAC cleanup was incomplete"}
	}
	result.Status = app.StatusSucceeded
	return result, nil
}
