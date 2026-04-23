package providers

import (
	"encoding/json"
	"strings"
)

func decodeOpenAITextContent(raw json.RawMessage) string {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return stripLeakedToolMarkup(extractTextFromStructuredContentString(text))
	}

	var parts []any
	if err := json.Unmarshal(raw, &parts); err == nil {
		return stripLeakedToolMarkup(extractTextFromOpenAIContentParts(parts))
	}

	var part map[string]any
	if err := json.Unmarshal(raw, &part); err == nil {
		return stripLeakedToolMarkup(extractTextFromOpenAIContentPart(part))
	}

	return stripLeakedToolMarkup(string(raw))
}

func extractTextFromOpenAIContentParts(parts []any) string {
	var b strings.Builder
	for _, part := range parts {
		m, ok := part.(map[string]any)
		if !ok {
			continue
		}
		b.WriteString(extractTextFromOpenAIContentPart(m))
	}
	return b.String()
}

func extractTextFromOpenAIContentPart(part map[string]any) string {
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

func extractTextFromStructuredContentString(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
		var parts []map[string]any
		if err := json.Unmarshal([]byte(trimmed), &parts); err == nil {
			items := make([]any, 0, len(parts))
			for _, p := range parts {
				items = append(items, p)
			}
			if text := extractTextFromOpenAIContentParts(items); text != "" {
				return text
			}
		}
		var part map[string]any
		if err := json.Unmarshal([]byte(trimmed), &part); err == nil {
			if text := extractTextFromOpenAIContentPart(part); text != "" {
				return text
			}
		}
	}

	if strings.HasPrefix(trimmed, "[{'type':") || strings.HasPrefix(trimmed, "{\"type\":") || strings.HasPrefix(trimmed, "[{\"type\":") {
		for _, marker := range []string{"'text': '", "\"text\":\""} {
			if idx := strings.Index(trimmed, marker); idx >= 0 {
				rest := trimmed[idx+len(marker):]
				if end := findQuotedTextEnd(rest, marker[0]); end >= 0 {
					return rest[:end]
				}
			}
		}
	}

	return s
}

func findQuotedTextEnd(s string, quote byte) int {
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

func bytesTrimSpace(b []byte) []byte {
	start := 0
	for start < len(b) && isSpace(b[start]) {
		start++
	}
	end := len(b)
	for end > start && isSpace(b[end-1]) {
		end--
	}
	return b[start:end]
}

func isSpace(c byte) bool {
	switch c {
	case ' ', '\n', '\r', '\t':
		return true
	default:
		return false
	}
}
