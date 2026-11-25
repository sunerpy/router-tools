package routerweb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/devices/xiaomi/common/webauth"
	"github.com/sunerpy/router-tools/internal/secret"
)

// Session owns an authenticated Web token without exposing it to presentation layers.
type Session struct {
	client *Client
	stok   string
}

// Authenticated reports whether the session still holds a token.
func (s *Session) Authenticated() bool { return s != nil && s.stok != "" }

// Close drops the token reference. Go cannot guarantee physical memory zeroization.
func (s *Session) Close() {
	if s != nil {
		s.stok = ""
	}
}

// String prevents accidental token disclosure through formatted output.
func (s *Session) String() string {
	return fmt.Sprintf("{authenticated:%t}", s.Authenticated())
}

// GoString prevents accidental token disclosure through %#v formatting.
func (s *Session) GoString() string { return s.String() }

// MarshalJSON exposes only authentication state, never stok.
func (s *Session) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Authenticated bool `json:"authenticated"`
	}{Authenticated: s.Authenticated()})
}

// Login performs Xiaomi Web authentication and returns an opaque session.
func (c *Client) Login(ctx context.Context, password secret.Password) (*Session, error) {
	if err := password.Validate(); err != nil {
		return nil, err
	}
	page, err := c.LoginPage(ctx)
	if err != nil {
		return nil, err
	}
	info, err := c.InitInfo(ctx)
	if err != nil {
		return nil, err
	}
	nonce, err := c.nonceFactory(page.DeviceID)
	if err != nil {
		return nil, fmt.Errorf("generate Xiaomi login nonce: %w", err)
	}
	passwordHash, err := webauth.PasswordHash(password.Expose(), page.Key, nonce, info.NewEncryptMode)
	if err != nil {
		return nil, &device.Error{Code: device.CodeWebProtocolError, Message: "cannot derive Xiaomi login hash"}
	}
	form := url.Values{
		"username": {"admin"},
		"password": {passwordHash},
		"logtype":  {"2"},
		"nonce":    {nonce},
	}
	body, err := c.postForm(ctx, "/cgi-bin/luci/api/xqsystem/login", form)
	if err != nil {
		return nil, err
	}
	token, err := parseLoginResponse(body)
	if err != nil {
		return nil, err
	}
	return &Session{client: c, stok: token}, nil
}

func parseLoginResponse(body []byte) (string, error) {
	object, err := decodeObject(body)
	if err != nil {
		return "", &device.Error{Code: device.CodeWebProtocolError, Message: "invalid Xiaomi login JSON response"}
	}
	rawCode, ok := object["code"]
	if !ok {
		return "", &device.Error{Code: device.CodeWebProtocolError, Message: "Xiaomi login response code is missing"}
	}
	code, err := decodeScalar(rawCode)
	if err != nil {
		return "", &device.Error{Code: device.CodeWebProtocolError, Message: "Xiaomi login response code is invalid"}
	}
	zero := false
	switch value := code.(type) {
	case json.Number:
		zero = value.String() == "0"
	case string:
		zero = value == "0"
	default:
		return "", &device.Error{Code: device.CodeWebProtocolError, Message: "Xiaomi login response code has an invalid type"}
	}
	if !zero {
		return "", &device.Error{Code: device.CodeWebAuthFailed, Message: "Xiaomi Web authentication was rejected"}
	}
	rawToken, ok := object["token"]
	if !ok {
		return "", &device.Error{Code: device.CodeWebProtocolError, Message: "Xiaomi login response token is missing"}
	}
	var token string
	if err := json.Unmarshal(rawToken, &token); err != nil || strings.TrimSpace(token) == "" {
		return "", &device.Error{Code: device.CodeWebProtocolError, Message: "Xiaomi login response token is empty or invalid"}
	}
	return token, nil
}
