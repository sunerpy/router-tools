package routerweb

import (
	"fmt"
	"regexp"
	"strings"
)

var loginPagePatterns = map[string][]*regexp.Regexp{
	"deviceId": {
		regexp.MustCompile(`\bvar\s+deviceId\s*=\s*'([^']*)'\s*;?`),
		regexp.MustCompile(`\bvar\s+deviceId\s*=\s*"([^"]*)"\s*;?`),
	},
	"key": {
		regexp.MustCompile(`\bkey\s*:\s*'([^']*)'\s*,`),
		regexp.MustCompile(`\bkey\s*:\s*"([^"]*)"\s*,`),
	},
}

// LoginPage contains fields required for Xiaomi Web authentication.
// Key is secret and must never be included in reports or logs.
type LoginPage struct {
	DeviceID string
	Key      string
}

// ParseLoginPage extracts exact JavaScript fields and rejects conflicting values.
func ParseLoginPage(body []byte) (LoginPage, error) {
	deviceID, err := extractConsistentField(body, "deviceId")
	if err != nil {
		return LoginPage{}, err
	}
	key, err := extractConsistentField(body, "key")
	if err != nil {
		return LoginPage{}, err
	}
	return LoginPage{DeviceID: deviceID, Key: key}, nil
}

func extractConsistentField(body []byte, field string) (string, error) {
	var value string
	found := false
	for _, pattern := range loginPagePatterns[field] {
		for _, match := range pattern.FindAllSubmatch(body, -1) {
			if len(match) != 2 || strings.TrimSpace(string(match[1])) == "" {
				return "", fmt.Errorf("login page field %s is empty", field)
			}
			candidate := string(match[1])
			if found && candidate != value {
				return "", fmt.Errorf("login page contains conflicting %s values", field)
			}
			value = candidate
			found = true
		}
	}
	if !found {
		return "", fmt.Errorf("login page field %s is missing", field)
	}
	return value, nil
}
