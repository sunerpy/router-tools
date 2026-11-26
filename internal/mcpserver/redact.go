package mcpserver

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/secret"
)

// sensitiveKeys are JSON member names whose string values are always hidden, like the CLI redactor's
// structured pass.
var sensitiveKeys = map[string]bool{"stok": true, "token": true, "key": true, "nonce": true}

// redactOutcome hides typed passwords in a finished result. Redaction works on string values only, so a
// short password such as "1" or "true" cannot corrupt the JSON structure.
func redactOutcome(redactor *secret.Redactor, result outcome) outcome {
	if redactor == nil {
		return result
	}
	if result.report != nil {
		result.report = redactValue(redactor, result.report)
	}
	if result.err != nil {
		copied := *result.err
		copied.Message = redactor.Text(copied.Message)
		copied.Stage = redactor.Text(copied.Stage)
		result.err = &copied
	}
	return result
}

func redactEvent(redactor *secret.Redactor, event app.Event) app.Event {
	if redactor == nil {
		return event
	}
	event.Operation = redactor.Text(event.Operation)
	event.Stage = redactor.Text(event.Stage)
	event.Status = redactor.Text(event.Status)
	event.Step = redactor.Text(event.Step)
	return event
}

// redactValue returns a JSON-equivalent copy of value with every string value redacted. Numbers keep their
// exact text.
func redactValue(redactor *secret.Redactor, value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var generic any
	if err := decoder.Decode(&generic); err != nil {
		return nil
	}
	return walk(redactor, generic, "")
}

func walk(redactor *secret.Redactor, value any, key string) any {
	switch typed := value.(type) {
	case string:
		if sensitiveKeys[strings.ToLower(key)] {
			return "<redacted>"
		}
		return redactor.Text(typed)
	case map[string]any:
		for name, member := range typed {
			typed[name] = walk(redactor, member, name)
		}
		return typed
	case []any:
		for index, element := range typed {
			typed[index] = walk(redactor, element, "")
		}
		return typed
	default:
		return value
	}
}

// newRedactor registers typed passwords, including their JSON-escaped forms, for one operation.
func newRedactor(values ...string) *secret.Redactor {
	redactor := secret.NewRedactor()
	for _, value := range values {
		if value == "" {
			continue
		}
		redactor.Add(value)
		if escaped := jsonEscaped(value); escaped != value {
			redactor.Add(escaped)
		}
	}
	return redactor
}

func jsonEscaped(value string) string {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	encoded := strings.TrimSuffix(buffer.String(), "\n")
	return strings.TrimSuffix(strings.TrimPrefix(encoded, `"`), `"`)
}
