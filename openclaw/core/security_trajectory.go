package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

func TrajectoryRedactPayload(text string, pipeline IRedactionPipeline) string {
	var v any
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	if err := decoder.Decode(&v); err != nil {
		return TrajectoryRedact(text, pipeline)
	}

	var sb strings.Builder
	encoder := json.NewEncoder(&sb)
	encoder.SetEscapeHTML(false)

	err := TrajectoryWriteRedactedJson(encoder, v, pipeline)
	if err != nil {
		return TrajectoryRedact(text, pipeline)
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

func TrajectoryRedact(text string, pipeline IRedactionPipeline) string {
	r := BaselineSecretRedactor{}
	return r.Redact(pipeline.Redact(text))
}

func TrajectoryWriteRedactedJson(encoder *json.Encoder, value any, pipeline IRedactionPipeline) error {
	switch val := value.(type) {
	case map[string]any:
		m := make(map[string]any)
		keys := make(map[string]bool, len(val))

		for k, v := range val {
			redactedKey := TrajectoryRedact(k, pipeline)
			if keys[redactedKey] {
				return fmt.Errorf("duplicate argument keys after redaction: %s", redactedKey)
			}
			keys[redactedKey] = true

			normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(k, "_", ""), "-", ""))
			switch normalized {
			case "password", "secret", "apikey", "token", "accesstoken", "refreshtoken", "authorization", "cookie", "clientsecret", "privatekey", "sessiontoken", "xapikey":
				m[redactedKey] = "[REDACTED]"
			default:
				processed, err := processValue(v, pipeline)
				if err != nil {
					return err
				}
				m[redactedKey] = processed
			}
		}
		return encoder.Encode(m)

	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			processed, err := processValue(item, pipeline)
			if err != nil {
				return err
			}
			result[i] = processed
		}
		return encoder.Encode(result)

	case string:
		return encoder.Encode(TrajectoryRedact(val, pipeline))

	default:
		rawBytes, _ := json.Marshal(val)
		raw := string(rawBytes)
		clean := TrajectoryRedact(raw, pipeline)
		if raw == clean {
			return encoder.Encode(val)
		}
		return encoder.Encode(clean)
	}
}

func processValue(value any, pipeline IRedactionPipeline) (any, error) {
	switch val := value.(type) {
	case map[string]any:
		m := make(map[string]any)
		keys := make(map[string]bool, len(val))
		for k, v := range val {
			redactedKey := TrajectoryRedact(k, pipeline)
			if keys[redactedKey] {
				return nil, fmt.Errorf("duplicate argument keys after redaction")
			}
			keys[redactedKey] = true

			normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(k, "_", ""), "-", ""))
			switch normalized {
			case "password", "secret", "apikey", "token", "accesstoken", "refreshtoken", "authorization", "cookie", "clientsecret", "privatekey", "sessiontoken", "xapikey":
				m[redactedKey] = "[REDACTED]"
			default:
				sub, err := processValue(v, pipeline)
				if err != nil {
					return nil, err
				}
				m[redactedKey] = sub
			}
		}
		return m, nil

	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			sub, err := processValue(item, pipeline)
			if err != nil {
				return nil, err
			}
			result[i] = sub
		}
		return result, nil

	case string:
		return TrajectoryRedact(val, pipeline), nil

	default:
		rawBytes, _ := json.Marshal(val)
		raw := string(rawBytes)
		clean := TrajectoryRedact(raw, pipeline)
		if raw == clean {
			return val, nil
		}
		return clean, nil
	}
}
