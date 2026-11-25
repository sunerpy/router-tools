package routerweb

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/sunerpy/router-tools/internal/device"
)

var macAddressPattern = regexp.MustCompile(`^[0-9A-F]{2}(:[0-9A-F]{2}){5}$`)

// AddMACRule submits one adapter-supplied fixed command through the authenticated MAC channel.
func (s *Session) AddMACRule(ctx context.Context, mac, command string) error {
	if err := validateMACRule(mac, command); err != nil {
		return err
	}
	form := url.Values{"mac": {mac}, "name": {"x$(" + command + ")y"}, "option": {"add"}, "wan": {""}}
	return s.sendMACRule(ctx, form, 20*time.Second, true)
}

// DeleteMACRule removes one attempted MAC rule. Cleanup retry policy belongs to the operation.
func (s *Session) DeleteMACRule(ctx context.Context, mac string) error {
	if !macAddressPattern.MatchString(mac) {
		return &device.Error{Code: device.CodeInvalidArgument, Message: "invalid MAC address"}
	}
	form := url.Values{"mac": {mac}, "name": {"x"}, "option": {"del"}, "wan": {""}}
	return s.sendMACRule(ctx, form, 8*time.Second, false)
}

func (s *Session) sendMACRule(ctx context.Context, form url.Values, timeout time.Duration, write bool) error {
	if !s.Authenticated() || s.client == nil {
		return &device.Error{Code: device.CodeWebAuthFailed, Message: "Web session is not authenticated"}
	}
	path := "/cgi-bin/luci/;stok=" + url.PathEscape(s.stok) + "/api/xqsystem/set_macfilter_rules"
	body, err := s.client.postFormWithTimeout(ctx, path, form, timeout)
	if err != nil {
		var statusError *HTTPStatusError
		if errors.As(err, &statusError) {
			return device.WrapError(device.CodeRemoteAPIRejected, "MAC rule request was rejected by HTTP status", err)
		}
		if strings.Contains(err.Error(), "response exceeds") {
			return device.WrapError(device.CodeWebProtocolError, "MAC rule response was invalid", err)
		}
		if write {
			return device.WrapError(device.CodeRemoteResultUnknown, "MAC add result is unknown; do not retry automatically", err)
		}
		return device.WrapError(device.CodeRemoteResultUnknown, "MAC delete transport failed", err)
	}
	if err := validateMACRuleResponse(body); err != nil {
		return err
	}
	return nil
}

func validateMACRule(mac, command string) error {
	if !macAddressPattern.MatchString(mac) {
		return &device.Error{Code: device.CodeInvalidArgument, Message: "invalid MAC address"}
	}
	if command == "" || strings.Contains(command, ";") {
		return &device.Error{Code: device.CodeInvalidArgument, Message: "MAC command must be non-empty and contain no semicolon"}
	}
	for _, r := range command {
		if r <= 0x1f || r == 0x7f {
			return &device.Error{Code: device.CodeInvalidArgument, Message: "MAC command contains a control character"}
		}
	}
	return nil
}

func validateMACRuleResponse(body []byte) error {
	object, err := decodeObject(body)
	if err != nil {
		return &device.Error{Code: device.CodeWebProtocolError, Message: "invalid MAC rule JSON response"}
	}
	rawCode, ok := object["code"]
	if !ok {
		return &device.Error{Code: device.CodeWebProtocolError, Message: "MAC rule response code is missing"}
	}
	code, err := decodeScalar(rawCode)
	if err != nil {
		return &device.Error{Code: device.CodeWebProtocolError, Message: "MAC rule response code is invalid"}
	}
	zero := false
	switch value := code.(type) {
	case json.Number:
		zero = value.String() == "0"
	case string:
		zero = value == "0"
	default:
		return &device.Error{Code: device.CodeWebProtocolError, Message: "MAC rule response code has an invalid type"}
	}
	if !zero {
		return &device.Error{Code: device.CodeRemoteAPIRejected, Message: "MAC rule request was rejected by the device API"}
	}
	return nil
}
