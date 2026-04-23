package gateway

import (
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/agent"
	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

func TestSanitizeLiveEventText_StripsStructuredContentString(t *testing.T) {
	in := `[{'type': 'text', 'text': 'Browser started successfully.'}]functions.browser:0{"action":"start"}`
	got := sanitizeLiveEventText(in)
	if got != "Browser started successfully." {
		t.Fatalf("sanitizeLiveEventText() = %q, want %q", got, "Browser started successfully.")
	}
}

func TestSanitizeEventPayload_ChatContent(t *testing.T) {
	ev := bus.Event{
		Name: protocol.EventChat,
		Payload: map[string]any{
			"type":    protocol.ChatEventChunk,
			"content": `[{"type":"text","text":"Checking geolocation"}]<|tool_call_argument_begin|>{"x":1}`,
		},
	}
	got, ok := sanitizeEventPayload(ev).(map[string]any)
	if !ok {
		t.Fatalf("payload type = %T, want map[string]any", sanitizeEventPayload(ev))
	}
	if got["content"] != "Checking geolocation" {
		t.Fatalf("content = %q, want %q", got["content"], "Checking geolocation")
	}
}

func TestSanitizeEventPayload_AgentNestedContent(t *testing.T) {
	ev := bus.Event{
		Name: protocol.EventAgent,
		Payload: agent.AgentEvent{
			Type: protocol.AgentEventRunCompleted,
			Payload: map[string]any{
				"content": `[{"type":"text","text":"Result ready"}]functions.browser:0{"action":"open"}`,
			},
		},
	}
	got, ok := sanitizeEventPayload(ev).(agent.AgentEvent)
	if !ok {
		t.Fatalf("payload type = %T, want agent.AgentEvent", sanitizeEventPayload(ev))
	}
	payload, _ := got.Payload.(map[string]any)
	if payload["content"] != "Result ready" {
		t.Fatalf("content = %q, want %q", payload["content"], "Result ready")
	}
}
