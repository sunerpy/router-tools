package secret

import (
	"encoding/base64"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var (
	querySecretPattern = regexp.MustCompile(`(?i)([?&](?:stok|token|key|nonce)=)[^&\s]+`)
	pathStokPattern    = regexp.MustCompile(`(?i)(;stok=)[^/\s]+`)
	jsonSecretPattern  = regexp.MustCompile(`(?i)("(?:stok|token|key|nonce)"\s*:\s*")[^"]*(")`)
)

// Redactor removes registered values and common Xiaomi secret fields from output.
type Redactor struct {
	mu     sync.RWMutex
	values map[string]struct{}
}

// NewRedactor constructs a redactor and registers optional secret values.
func NewRedactor(values ...string) *Redactor {
	redactor := &Redactor{values: make(map[string]struct{})}
	redactor.Add(values...)
	return redactor
}

// Add registers raw, Base64, and URL-escaped representations. Empty values are ignored.
func (r *Redactor) Add(values ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, value := range values {
		if value == "" {
			continue
		}
		r.values[value] = struct{}{}
		r.values[base64.StdEncoding.EncodeToString([]byte(value))] = struct{}{}
		r.values[url.QueryEscape(value)] = struct{}{}
		r.values[url.PathEscape(value)] = struct{}{}
	}
}

// Text redacts structured secret fields before replacing registered literals.
func (r *Redactor) Text(input string) string {
	if input == "" {
		return input
	}
	output := querySecretPattern.ReplaceAllString(input, `${1}<redacted>`)
	output = pathStokPattern.ReplaceAllString(output, `${1}<redacted>`)
	output = jsonSecretPattern.ReplaceAllString(output, `${1}<redacted>${2}`)

	r.mu.RLock()
	values := make([]string, 0, len(r.values))
	for value := range r.values {
		values = append(values, value)
	}
	r.mu.RUnlock()
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	for _, value := range values {
		output = strings.ReplaceAll(output, value, "<redacted>")
	}
	return output
}
