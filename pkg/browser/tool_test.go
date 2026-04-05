package browser

import (
	"errors"
	"strings"
	"testing"
)

func TestBrowserStartHintPinchTabPermissionDenied(t *testing.T) {
	err := errors.New("pinchtab instance abc unhealthy: permission denied while launching chrome")

	got := browserStartHint(err)
	if got == "" {
		t.Fatal("expected hint, got empty string")
	}
	if !strings.Contains(got, "PINCHTAB_CHROME_NO_SANDBOX=1") {
		t.Fatalf("expected no-sandbox hint, got %q", got)
	}
}

func TestBrowserStartHintNonPermissionDenied(t *testing.T) {
	err := errors.New("pinchtab HTTP 401 /profiles: unauthorized")

	if got := browserStartHint(err); got != "" {
		t.Fatalf("expected no hint, got %q", got)
	}
}

func TestFormatBrowserStartErrorIncludesHint(t *testing.T) {
	err := errors.New("launch chrome: permission denied")

	got := formatBrowserStartError(err)
	if !strings.Contains(got, "failed to start browser") {
		t.Fatalf("expected formatted prefix, got %q", got)
	}
	if !strings.Contains(got, "[BROWSER]") {
		t.Fatalf("expected browser hint marker, got %q", got)
	}
}
