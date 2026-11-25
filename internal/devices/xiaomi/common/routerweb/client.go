package routerweb

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/devices/xiaomi/common/webauth"
)

const (
	requestTimeout  = 5 * time.Second
	maxResponseSize = 1 << 20
)

// NonceFactory supplies deterministic nonces in tests and crypto/rand nonces in production.
type NonceFactory func(deviceID string) (string, error)

// ClientOptions contains protocol dependencies that do not change runtime policy.
type ClientOptions struct {
	NonceFactory NonceFactory
}

// Client is a fixed-endpoint Xiaomi Web client. It has no arbitrary command API.
type Client struct {
	baseURL      string
	http         *http.Client
	nonceFactory NonceFactory
}

// NewClient accepts only plain HTTP base URLs without credentials, paths, queries, or fragments.
func NewClient(rawBaseURL string) (*Client, error) {
	return NewClientWithOptions(rawBaseURL, ClientOptions{})
}

// NewClientWithOptions constructs a client with an optional deterministic nonce factory.
func NewClientWithOptions(rawBaseURL string, options ClientOptions) (*Client, error) {
	parsed, err := url.Parse(rawBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Web base URL: %w", err)
	}
	if parsed.Scheme != "http" || parsed.Host == "" {
		return nil, fmt.Errorf("web base URL must use http and include a host")
	}
	if parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("web base URL must not contain credentials, a path, query, or fragment")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create Web cookie jar: %w", err)
	}
	nonceFactory := options.NonceFactory
	if nonceFactory == nil {
		nonceFactory = func(deviceID string) (string, error) {
			return webauth.GenerateNonce(deviceID, time.Now(), rand.Reader)
		}
	}
	return &Client{
		baseURL:      strings.TrimSuffix(rawBaseURL, "/"),
		nonceFactory: nonceFactory,
		http: &http.Client{
			Timeout: requestTimeout,
			Jar:     jar,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// Probe performs only the two GET requests required for device identification.
// The Web key is deliberately discarded after validating the login page.
func (c *Client) Probe(ctx context.Context) (device.Observation, error) {
	page, err := c.LoginPage(ctx)
	if err != nil {
		return device.Observation{}, err
	}
	info, err := c.InitInfo(ctx)
	if err != nil {
		return device.Observation{}, err
	}
	return device.Observation{
		DeviceID:       page.DeviceID,
		Hardware:       info.Hardware,
		DisplayName:    info.DisplayName,
		ROMVersion:     info.ROMVersion,
		NewEncryptMode: int(info.NewEncryptMode),
	}, nil
}

// LoginPage fetches and parses the fixed Xiaomi login page.
func (c *Client) LoginPage(ctx context.Context) (LoginPage, error) {
	body, err := c.get(ctx, "/cgi-bin/luci/web")
	if err != nil {
		return LoginPage{}, err
	}
	page, err := ParseLoginPage(body)
	if err != nil {
		return LoginPage{}, fmt.Errorf("parse Xiaomi login page: %w", err)
	}
	return page, nil
}

// InitInfo fetches and parses the fixed Xiaomi device information endpoint.
func (c *Client) InitInfo(ctx context.Context) (InitInfo, error) {
	body, err := c.get(ctx, "/cgi-bin/luci/api/xqsystem/init_info")
	if err != nil {
		return InitInfo{}, err
	}
	info, err := ParseInitInfo(body)
	if err != nil {
		return InitInfo{}, fmt.Errorf("parse Xiaomi init_info: %w", err)
	}
	return info, nil
}

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, path, nil, "")
}

func (c *Client) postForm(ctx context.Context, path string, form url.Values) ([]byte, error) {
	return c.postFormWithTimeout(ctx, path, form, requestTimeout)
}

func (c *Client) postFormWithTimeout(ctx context.Context, path string, form url.Values, timeout time.Duration) ([]byte, error) {
	return c.doWithTimeout(ctx, http.MethodPost, path, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", timeout)
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string) ([]byte, error) {
	return c.doWithTimeout(ctx, method, path, body, contentType, requestTimeout)
}

func (c *Client) doWithTimeout(ctx context.Context, method, path string, body io.Reader, contentType string, timeout time.Duration) ([]byte, error) {
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("create Xiaomi Web request: %w", err)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("xiaomi Web request failed: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, &HTTPStatusError{StatusCode: response.StatusCode}
	}
	limited := io.LimitReader(response.Body, maxResponseSize+1)
	responseBody, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read Xiaomi Web response: %w", err)
	}
	if len(responseBody) > maxResponseSize {
		return nil, fmt.Errorf("xiaomi Web response exceeds %d bytes", maxResponseSize)
	}
	return responseBody, nil
}

// HTTPStatusError reports a completed HTTP request with a non-2xx status.
type HTTPStatusError struct {
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("xiaomi Web endpoint returned HTTP %d", e.StatusCode)
}
