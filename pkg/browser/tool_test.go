package browser

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
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

type timeoutProbeBackend struct {
	lastCtx context.Context
}

func (b *timeoutProbeBackend) Start(ctx context.Context) error                           { return nil }
func (b *timeoutProbeBackend) Stop(ctx context.Context) error                            { return nil }
func (b *timeoutProbeBackend) Status() *StatusInfo                                       { return &StatusInfo{} }
func (b *timeoutProbeBackend) ListTabs(ctx context.Context) ([]TabInfo, error)           { return nil, nil }
func (b *timeoutProbeBackend) OpenTab(ctx context.Context, url string) (*TabInfo, error) { b.lastCtx = ctx; return nil, context.DeadlineExceeded }
func (b *timeoutProbeBackend) CloseTab(ctx context.Context, targetID string) error       { return nil }
func (b *timeoutProbeBackend) ActionTimeout() time.Duration                              { return 120 * time.Second }
func (b *timeoutProbeBackend) ConsoleMessages(ctx context.Context, targetID string) []ConsoleMessage {
	return nil
}
func (b *timeoutProbeBackend) Snapshot(ctx context.Context, targetID string, opts SnapshotOptions) (*SnapshotResult, error) {
	return nil, nil
}
func (b *timeoutProbeBackend) Screenshot(ctx context.Context, targetID string, fullPage bool) ([]byte, error) {
	return nil, nil
}
func (b *timeoutProbeBackend) Navigate(ctx context.Context, targetID string, url string) error { return nil }
func (b *timeoutProbeBackend) Click(ctx context.Context, targetID string, ref string, opts ClickOpts) error {
	return nil
}
func (b *timeoutProbeBackend) Type(ctx context.Context, targetID string, ref string, text string, opts TypeOpts) error {
	return nil
}
func (b *timeoutProbeBackend) Press(ctx context.Context, targetID string, key string) error { return nil }
func (b *timeoutProbeBackend) Hover(ctx context.Context, targetID string, ref string) error  { return nil }
func (b *timeoutProbeBackend) Wait(ctx context.Context, targetID string, opts WaitOpts) error { return nil }
func (b *timeoutProbeBackend) Evaluate(ctx context.Context, targetID string, fn string) (string, error) {
	return "", nil
}

func TestBrowserToolOpenTimeoutHonorsBackendFloor(t *testing.T) {
	be := &timeoutProbeBackend{}
	tool := NewBrowserTool(be)

	res := tool.Execute(context.Background(), map[string]any{
		"action":    "open",
		"targetUrl": "https://example.com",
		"timeoutMs": float64(30000),
	})
	if res == nil || res.IsError == false {
		t.Fatalf("expected error result from timeout probe, got %#v", res)
	}
	if be.lastCtx == nil {
		t.Fatal("expected backend to receive a context")
	}
	deadline, ok := be.lastCtx.Deadline()
	if !ok {
		t.Fatal("expected deadline on backend context")
	}
	remaining := time.Until(deadline)
	if remaining < 110*time.Second {
		t.Fatalf("expected backend timeout floor near 120s, got %v", remaining)
	}
	if !strings.Contains(fmt.Sprint(res.ForLLM), "context deadline exceeded") {
		t.Fatalf("expected timeout error in result, got %#v", res.ForLLM)
	}
}
