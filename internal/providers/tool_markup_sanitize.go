package providers

import "strings"

var leakedToolMarkupMarkers = []string{
	"<|tool_call_begin|>",
	"<|tool_call_end|>",
	"<|tool_call_argument_begin|>",
	"<|tool_call_argument_end|>",
}

// stripLeakedToolMarkup removes provider-specific tool-call control markup
// that some OpenAI-compatible models incorrectly emit as plain assistant text.
// If a tool marker appears inside a content chunk, we keep only the natural
// language prefix before the marker and drop the synthetic suffix.
func stripLeakedToolMarkup(s string) string {
	if s == "" {
		return s
	}
	cut := -1
	for _, marker := range leakedToolMarkupMarkers {
		if idx := strings.Index(s, marker); idx >= 0 && (cut == -1 || idx < cut) {
			cut = idx
		}
	}
	if cut >= 0 {
		s = s[:cut]
	}
	for _, marker := range leakedToolMarkupMarkers {
		s = strings.ReplaceAll(s, marker, "")
	}
	return strings.TrimRight(s, " \t\r\n")
}
