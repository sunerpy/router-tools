package routerweb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/sunerpy/router-tools/internal/devices/xiaomi/common/webauth"
)

// InitInfo is the public subset of Xiaomi's init_info response.
type InitInfo struct {
	DisplayName    string
	Hardware       string
	ROMVersion     string
	NewEncryptMode webauth.EncryptionMode
}

// ParseInitInfo validates the API envelope and encryption mode without weak-algorithm fallback.
func ParseInitInfo(body []byte) (InitInfo, error) {
	object, err := decodeObject(body)
	if err != nil {
		return InitInfo{}, fmt.Errorf("decode init_info: %w", err)
	}
	if err := validateAPICode(object); err != nil {
		return InitInfo{}, fmt.Errorf("validate init_info: %w", err)
	}

	modeValue, ok := object["newEncryptMode"]
	if !ok {
		return InitInfo{}, fmt.Errorf("init_info.newEncryptMode is missing")
	}
	modeNumber, err := decodeInteger(modeValue)
	if err != nil {
		return InitInfo{}, fmt.Errorf("init_info.newEncryptMode: %w", err)
	}
	mode := webauth.EncryptionSHA1
	if modeNumber == 1 {
		mode = webauth.EncryptionSHA256
	}

	displayName, err := optionalString(object, "displayName")
	if err != nil {
		return InitInfo{}, err
	}
	hardware, err := optionalString(object, "hardware")
	if err != nil {
		return InitInfo{}, err
	}
	romVersion, err := optionalString(object, "romversion")
	if err != nil {
		return InitInfo{}, err
	}
	return InitInfo{
		DisplayName:    displayName,
		Hardware:       hardware,
		ROMVersion:     romVersion,
		NewEncryptMode: mode,
	}, nil
}

func decodeObject(body []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var object map[string]json.RawMessage
	if err := decoder.Decode(&object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("top-level JSON value is not an object")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple top-level JSON values")
		}
		return nil, err
	}
	return object, nil
}

func validateAPICode(object map[string]json.RawMessage) error {
	raw, ok := object["code"]
	if !ok {
		return fmt.Errorf("API code is missing")
	}
	value, err := decodeScalar(raw)
	if err != nil {
		return fmt.Errorf("API code: %w", err)
	}
	switch code := value.(type) {
	case json.Number:
		if code.String() == "0" {
			return nil
		}
	case string:
		if code == "0" {
			return nil
		}
	}
	return fmt.Errorf("API request was rejected")
}

func decodeInteger(raw json.RawMessage) (int, error) {
	value, err := decodeScalar(raw)
	if err != nil {
		return 0, err
	}
	var text string
	switch number := value.(type) {
	case json.Number:
		text = number.String()
	case string:
		text = number
	default:
		return 0, fmt.Errorf("expected integer or decimal string")
	}
	parsed, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("expected integer or decimal string")
	}
	return parsed, nil
}

func decodeScalar(raw json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func optionalString(object map[string]json.RawMessage, field string) (string, error) {
	raw, ok := object[field]
	if !ok {
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("init_info.%s must be a string", field)
	}
	return value, nil
}
