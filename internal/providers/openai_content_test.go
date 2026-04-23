package providers

import (
	"encoding/json"
	"testing"
)

func TestOpenAIMessage_UnmarshalMultipartContentArray(t *testing.T) {
	var msg openAIMessage
	raw := []byte(`{
		"role":"assistant",
		"content":[
			{"type":"text","text":"Hello"},
			{"type":"output_text","text":" world"},
			{"type":"image_url","image_url":{"url":"data:image/png;base64,abc"}}
		]
	}`)
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if msg.Content != "Hello world" {
		t.Fatalf("Content = %q, want %q", msg.Content, "Hello world")
	}
}

func TestOpenAIMessage_UnmarshalStructuredContentString(t *testing.T) {
	var msg openAIMessage
	raw := []byte(`{
		"role":"assistant",
		"content":"[{\"type\":\"text\",\"text\":\"Browser opened\"}]"
	}`)
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if msg.Content != "Browser opened" {
		t.Fatalf("Content = %q, want %q", msg.Content, "Browser opened")
	}
}

func TestOpenAIMessage_UnmarshalSingleQuotedStructuredString(t *testing.T) {
	var msg openAIMessage
	raw := []byte(`{
		"role":"assistant",
		"content":"[{'type': 'text', 'text': 'I will check geolocation now.'}]"
	}`)
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if msg.Content != "I will check geolocation now." {
		t.Fatalf("Content = %q, want %q", msg.Content, "I will check geolocation now.")
	}
}

func TestOpenAIStreamDelta_UnmarshalMultipartContentArray(t *testing.T) {
	var delta openAIStreamDelta
	raw := []byte(`{
		"content":[
			{"type":"text","text":"Checking "},
			{"type":"output_text","text":"location"}
		]
	}`)
	if err := json.Unmarshal(raw, &delta); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if delta.Content != "Checking location" {
		t.Fatalf("Content = %q, want %q", delta.Content, "Checking location")
	}
}

func TestOpenAIAdapter_FromStreamChunk_SanitizesMultipartContent(t *testing.T) {
	a, _ := NewOpenAIAdapter(ProviderConfig{APIKey: "k"})
	raw := []byte(`{"choices":[{"delta":{"content":[{"type":"text","text":"Done"},{"type":"output_text","text":" now"}]}}]}`)
	sc, err := a.FromStreamChunk(raw)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if sc == nil || sc.Content != "Done now" {
		t.Fatalf("StreamChunk = %+v, want Content=%q", sc, "Done now")
	}
}
