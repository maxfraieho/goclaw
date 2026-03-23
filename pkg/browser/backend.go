package browser

import "context"

// Backend is the browser automation interface implemented by Manager (go-rod/CDP)
// and PinchTabManager (PinchTab HTTP API).
type Backend interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Status() *StatusInfo
	ListTabs(ctx context.Context) ([]TabInfo, error)
	OpenTab(ctx context.Context, url string) (*TabInfo, error)
	CloseTab(ctx context.Context, targetID string) error
	ConsoleMessages(targetID string) []ConsoleMessage
	Snapshot(ctx context.Context, targetID string, opts SnapshotOptions) (*SnapshotResult, error)
	Screenshot(ctx context.Context, targetID string, fullPage bool) ([]byte, error)
	Navigate(ctx context.Context, targetID string, url string) error
	Click(ctx context.Context, targetID string, ref string, opts ClickOpts) error
	Type(ctx context.Context, targetID string, ref string, text string, opts TypeOpts) error
	Press(ctx context.Context, targetID string, key string) error
	Hover(ctx context.Context, targetID string, ref string) error
	Wait(ctx context.Context, targetID string, opts WaitOpts) error
	Evaluate(ctx context.Context, targetID string, fn string) (string, error)
}
