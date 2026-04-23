package gateway

import (
	"encoding/json"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/agent"
	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// sanitizeEventPayload normalizes live WS event payloads before they are sent to
// browser clients. This protects the live chat UI from provider artifacts that
// may still appear in streamed events even when the persisted final assistant
// message is already sanitized.
func sanitizeEventPayload(event bus.Event) any {
	switch event.Name {
	case protocol.EventChat:
		return sanitizeChatEventPayload(event.Payload)
	case protocol.EventAgent:
		return sanitizeAgentEventPayload(event.Payload)
	default:
		return event.Payload
	}
}

func sanitizeChatEventPayload(payload any) any {
	switch p := payload.(type) {
	case map[string]any:
		return sanitizeStringFieldsMap(p, "content")
	case map[string]string:
		out := make(map[string]string, len(p))
		for k, v := range p {
			if k == "content" {
				out[k] = sanitizeLiveEventText(v)
			} else {
				out[k] = v
			}
		}
		return out
	default:
		return payload
	}
}

func sanitizeAgentEventPayload(payload any) any {
	switch p := payload.(type) {
	case agent.AgentEvent:
		p.Payload = sanitizeNestedPayload(p.Payload)
		return p
	case *agent.AgentEvent:
		cp := *p
		cp.Payload = sanitizeNestedPayload(p.Payload)
		return cp
	case map[string]any:
		return sanitizeNestedPayload(p)
	default:
		return payload
	}
}

func sanitizeNestedPayload(payload any) any {
	switch p := payload.(type) {
	case map[string]any:
		return sanitizeStringFieldsMap(p, "content", "thinking", "message")
	case map[string]string:
		out := make(map[string]string, len(p))
		for k, v := range p {
			switch k {
			case "content", "thinking", "message":
				out[k] = sanitizeLiveEventText(v)
			default:
				out[k] = v
			}
		}
		return out
	default:
		return payload
	}
}

func sanitizeStringFieldsMap(in map[string]any, keys ...string) map[string]any {
	keySet := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		keySet[k] = struct{}{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		switch vv := v.(type) {
		case string:
			if _, ok := keySet[k]; ok {
				out[k] = sanitizeLiveEventText(vv)
			} else {
				out[k] = vv
			}
		case map[string]any:
			out[k] = sanitizeStringFieldsMap(vv, keys...)
		default:
			out[k] = v
		}
	}
	return out
}

func sanitizeLiveEventText(s string) string {
	s = stripLeakedToolMarkupLive(s)
	s = extractTextFromStructuredContentStringLive(s)
	s = agent.SanitizeAssistantContent(s)
	return strings.TrimSpace(s)
}

func stripLeakedToolMarkupLive(s string) string {
	if s == "" {
		return s
	}
	markers := []string{
		"<|tool_call_begin|>",
		"<|tool_call_end|>",
		"<|tool_call_argument_begin|>",
		"<|tool_call_argument_end|>",
		"functions.",
	}
	cut := len(s)
	for _, marker := range markers {
		if idx := strings.Index(s, marker); idx >= 0 && idx < cut {
			cut = idx
		}
	}
	return strings.TrimSpace(s[:cut])
}

func extractTextFromStructuredContentStringLive(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
		var parts []map[string]any
		if err := json.Unmarshal([]byte(trimmed), &parts); err == nil {
			if text := extractTextFromPartsLive(parts); text != "" {
				return text
			}
		}
		var part map[string]any
		if err := json.Unmarshal([]byte(trimmed), &part); err == nil {
			if text := extractTextFromPartLive(part); text != "" {
				return text
			}
		}
	}

	if strings.HasPrefix(trimmed, "[{'type':") || strings.HasPrefix(trimmed, "[{\"type\":") || strings.HasPrefix(trimmed, "{\"type\":") {
		for _, marker := range []string{"'text': '", "\"text\":\""} {
			if idx := strings.Index(trimmed, marker); idx >= 0 {
				rest := trimmed[idx+len(marker):]
				if end := findQuotedTextEndLive(rest, marker[0]); end >= 0 {
					return rest[:end]
				}
			}
		}
	}

	return s
}

func extractTextFromPartsLive(parts []map[string]any) string {
	var b strings.Builder
	for _, part := range parts {
		b.WriteString(extractTextFromPartLive(part))
	}
	return b.String()
}

func extractTextFromPartLive(part map[string]any) string {
	partType, _ := part["type"].(string)
	switch partType {
	case "", "text", "output_text", "input_text":
		if text, ok := part["text"].(string); ok {
			return text
		}
		if textObj, ok := part["text"].(map[string]any); ok {
			if value, ok := textObj["value"].(string); ok {
				return value
			}
		}
	}
	return ""
}

func findQuotedTextEndLive(s string, quote byte) int {
	escaped := false
	for i := 0; i < len(s); i++ {
		switch {
		case escaped:
			escaped = false
		case s[i] == '\\':
			escaped = true
		case s[i] == quote:
			return i
		}
	}
	return -1
}
