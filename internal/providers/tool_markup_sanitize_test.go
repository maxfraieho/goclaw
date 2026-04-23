package providers

import "testing"

func TestStripLeakedToolMarkup_RemovesSyntheticSuffix(t *testing.T) {
	in := "Тепер зроблю скріншот, щоб побачити результат<|tool_call_argument_begin|>{\"action\":\"screenshot\"}"
	got := stripLeakedToolMarkup(in)
	want := "Тепер зроблю скріншот, щоб побачити результат"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestStripLeakedToolMarkup_StripsStandaloneMarkers(t *testing.T) {
	in := "<|tool_call_begin|><|tool_call_end|>"
	got := stripLeakedToolMarkup(in)
	if got != "" {
		t.Fatalf("got %q, want empty string", got)
	}
}
