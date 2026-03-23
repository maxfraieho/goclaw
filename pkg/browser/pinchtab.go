package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// PinchTabManager implements Backend via the PinchTab HTTP API.
// PinchTab is a standalone Go binary (~15MB) that runs as a local daemon
// at http://localhost:9867 and gives AI agents token-efficient browser control.
//
// Token economy: PinchTab snapshots use ~800 tokens/page (5–13× cheaper than screenshots
// and 10× cheaper than raw HTML). Snapshots with filter=interactive are even smaller.
//
// Docs: https://pinchtab.com/docs
// Repo: https://github.com/pinchtab/pinchtab
type PinchTabManager struct {
	mu         sync.Mutex
	baseURL    string      // e.g. "http://localhost:9867"
	token      string      // Bearer token for Authorization header (optional)
	client     *http.Client
	profileID  string
	instanceID string
	logger     *slog.Logger
}

// NewPinchTabManager creates a manager that delegates to a PinchTab server.
// baseURL is the PinchTab server address, e.g. "http://localhost:9867".
// token is an optional Bearer token for API authentication.
func NewPinchTabManager(baseURL, token string) *PinchTabManager {
	return &PinchTabManager{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: 30 * time.Second},
		logger:  slog.Default(),
	}
}

// ── PinchTab API response types ───────────────────────────────────────────────

type ptProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ptInstance struct {
	ID   string `json:"id"`
	Mode string `json:"mode"`
}

type ptInstancesResp struct {
	Instances []ptInstance `json:"instances"`
}

type ptTabOpenResp struct {
	TabID string `json:"tabId"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

type ptTabInfo struct {
	TabID string `json:"tabId"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

type ptTabsResp struct {
	Tabs []ptTabInfo `json:"tabs"`
}

type ptSnapshotNode struct {
	Ref      string `json:"ref"`
	Role     string `json:"role"`
	Name     string `json:"name"`
	Depth    int    `json:"depth"`
	Checked  *bool  `json:"checked,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
	Value    string `json:"value,omitempty"`
}

type ptSnapshotResp struct {
	Snapshot string           `json:"snapshot"` // legacy text format
	Nodes    []ptSnapshotNode `json:"nodes"`    // v0.8.x structured format
	URL      string           `json:"url"`
	Title    string           `json:"title"`
	TabID    string           `json:"tabId"`
}

// buildSnapshotText converts structured nodes to the text format expected by parseRefsFromSnapshot.
// Format per line: "  - role "name" [ref=eN]" with depth-based indentation.
func buildSnapshotText(nodes []ptSnapshotNode) string {
	var sb strings.Builder
	for _, n := range nodes {
		indent := strings.Repeat("  ", n.Depth)
		line := indent + "- " + n.Role
		if n.Name != "" {
			line += " " + fmt.Sprintf("%q", n.Name)
		}
		if n.Value != "" {
			line += " value=" + fmt.Sprintf("%q", n.Value)
		}
		if n.Checked != nil {
			if *n.Checked {
				line += " [checked]"
			} else {
				line += " [unchecked]"
			}
		}
		if n.Disabled {
			line += " [disabled]"
		}
		if n.Ref != "" {
			line += " [ref=" + n.Ref + "]"
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

type ptActionResp struct {
	OK     bool   `json:"ok"`
	Result string `json:"result,omitempty"`
}

type ptConsoleResp struct {
	Messages []ConsoleMessage `json:"messages"`
}

// ── Lifecycle ─────────────────────────────────────────────────────────────────

// Start creates a default profile and starts a headless Chrome instance in PinchTab.
// Idempotent: if a healthy instance already exists, it is reused.
func (p *PinchTabManager) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Reuse existing instance if healthy
	if p.instanceID != "" {
		if _, err := p.getInstanceLocked(p.instanceID); err == nil {
			return nil
		}
		p.logger.Info("pinchtab: instance gone, recreating", "instance", p.instanceID)
		p.instanceID = ""
		p.profileID = ""
	}

	// Create a profile for GoClaw; if it already exists (409 from unclean shutdown), reuse it.
	prof, err := p.doPost(ctx, "/profiles", map[string]any{"name": "goclaw"})
	if err != nil {
		if !strings.Contains(err.Error(), "409") {
			return fmt.Errorf("pinchtab create profile: %w", err)
		}
		// Profile exists from previous session — find it by name.
		all, listErr := p.doGet(ctx, "/profiles")
		if listErr != nil {
			return fmt.Errorf("pinchtab create profile: %w (list fallback: %v)", err, listErr)
		}
		var profiles []ptProfile
		if jsonErr := json.Unmarshal(all, &profiles); jsonErr != nil {
			return fmt.Errorf("pinchtab profile list decode: %w", jsonErr)
		}
		var found bool
		for _, pp := range profiles {
			if pp.Name == "goclaw" {
				p.profileID = pp.ID
				found = true
				p.logger.Info("pinchtab: reusing existing profile", "profile", p.profileID)
				break
			}
		}
		if !found {
			return fmt.Errorf("pinchtab create profile: %w (profile not found after 409)", err)
		}
	} else {
		var profResp ptProfile
		if err := json.Unmarshal(prof, &profResp); err != nil {
			return fmt.Errorf("pinchtab profile decode: %w", err)
		}
		p.profileID = profResp.ID
	}

	// Start headless instance for that profile
	inst, err := p.doPost(ctx, "/instances/start", map[string]any{
		"profileId": p.profileID,
		"mode":      "headless",
	})
	if err != nil {
		return fmt.Errorf("pinchtab start instance: %w", err)
	}
	var instResp ptInstance
	if err := json.Unmarshal(inst, &instResp); err != nil {
		return fmt.Errorf("pinchtab instance decode: %w", err)
	}
	p.instanceID = instResp.ID
	p.logger.Info("pinchtab: instance started", "instance", p.instanceID, "profile", p.profileID)
	return nil
}

// Stop disconnects from PinchTab. The daemon itself keeps running.
// PinchTab is designed as a persistent local daemon — we just release our instance.
func (p *PinchTabManager) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.instanceID == "" {
		return nil
	}
	// Best-effort stop; ignore errors (daemon stays alive)
	_, _ = p.doDelete(ctx, "/instances/"+p.instanceID)
	if p.profileID != "" {
		_, _ = p.doDelete(ctx, "/profiles/"+p.profileID)
	}
	p.instanceID = ""
	p.profileID = ""
	return nil
}

// Status returns whether PinchTab is running and how many tabs are open.
func (p *PinchTabManager) Status() *StatusInfo {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.instanceID == "" {
		return &StatusInfo{Running: false}
	}

	tabs, err := p.listTabsLocked(context.Background())
	if err != nil {
		return &StatusInfo{Running: false}
	}

	info := &StatusInfo{Running: true, Tabs: len(tabs)}
	if len(tabs) > 0 {
		info.URL = tabs[0].URL
	}
	return info
}

// ── Tabs ──────────────────────────────────────────────────────────────────────

func (p *PinchTabManager) ListTabs(ctx context.Context) ([]TabInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.listTabsLocked(ctx)
}

func (p *PinchTabManager) listTabsLocked(ctx context.Context) ([]TabInfo, error) {
	if p.instanceID == "" {
		return nil, fmt.Errorf("pinchtab: not started")
	}
	data, err := p.doGet(ctx, "/instances/"+p.instanceID+"/tabs")
	if err != nil {
		return nil, err
	}
	var resp ptTabsResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("pinchtab tabs decode: %w", err)
	}
	result := make([]TabInfo, len(resp.Tabs))
	for i, t := range resp.Tabs {
		result[i] = TabInfo{TargetID: t.TabID, URL: t.URL, Title: t.Title}
	}
	return result, nil
}

func (p *PinchTabManager) OpenTab(ctx context.Context, url string) (*TabInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.instanceID == "" {
		return nil, fmt.Errorf("pinchtab: not started")
	}
	data, err := p.doPost(ctx, "/instances/"+p.instanceID+"/tabs/open", map[string]any{"url": url})
	if err != nil {
		return nil, fmt.Errorf("pinchtab open tab: %w", err)
	}
	var resp ptTabOpenResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("pinchtab tab open decode: %w", err)
	}
	return &TabInfo{TargetID: resp.TabID, URL: resp.URL, Title: resp.Title}, nil
}

func (p *PinchTabManager) CloseTab(ctx context.Context, targetID string) error {
	_, err := p.doDelete(ctx, "/tabs/"+targetID)
	return err
}

// ConsoleMessages returns captured console messages for a tab.
// PinchTab tracks console output since v0.8 (feat: add console and errors tracking).
func (p *PinchTabManager) ConsoleMessages(targetID string) []ConsoleMessage {
	data, err := p.doGet(context.Background(), "/tabs/"+targetID+"/console")
	if err != nil {
		return []ConsoleMessage{}
	}
	var resp ptConsoleResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return []ConsoleMessage{}
	}
	return resp.Messages
}

// ── Page operations ───────────────────────────────────────────────────────────

// Snapshot returns a token-efficient accessibility snapshot from PinchTab.
// With filter=interactive: ~800 tokens/page (5–13× cheaper than screenshots).
// With filter=all: full accessibility tree, still more compact than raw HTML.
func (p *PinchTabManager) Snapshot(ctx context.Context, targetID string, opts SnapshotOptions) (*SnapshotResult, error) {
	filter := "all"
	if opts.Interactive {
		filter = "interactive"
	}

	data, err := p.doGet(ctx, fmt.Sprintf("/tabs/%s/snapshot?filter=%s", targetID, filter))
	if err != nil {
		return nil, fmt.Errorf("pinchtab snapshot: %w", err)
	}
	var resp ptSnapshotResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("pinchtab snapshot decode: %w", err)
	}

	// PinchTab v0.8.x returns structured nodes; older versions returned snapshot text directly.
	snap := resp.Snapshot
	if snap == "" && len(resp.Nodes) > 0 {
		snap = buildSnapshotText(resp.Nodes)
	}
	if opts.MaxChars > 0 && len(snap) > opts.MaxChars {
		snap = snap[:opts.MaxChars] + "\n[...TRUNCATED]"
	}

	refs, interactiveCount := parseRefsFromSnapshot(snap)
	return &SnapshotResult{
		Snapshot: snap,
		Refs:     refs,
		URL:      resp.URL,
		Title:    resp.Title,
		TargetID: targetID,
		Stats: SnapshotStats{
			Lines:       strings.Count(snap, "\n") + 1,
			Chars:       len(snap),
			Refs:        len(refs),
			Interactive: interactiveCount,
		},
	}, nil
}

// parseRefsFromSnapshot extracts ref names from PinchTab snapshot text.
// PinchTab uses the same [ref=eN] format as Playwright/GoClaw accessibility snapshots.
func parseRefsFromSnapshot(snapshot string) (map[string]RoleRef, int) {
	refs := make(map[string]RoleRef)
	interactive := 0
	for _, line := range strings.Split(snapshot, "\n") {
		start := strings.Index(line, "[ref=")
		if start < 0 {
			continue
		}
		end := strings.Index(line[start:], "]")
		if end < 0 {
			continue
		}
		ref := line[start+5 : start+end]
		// Extract role from line: "  - button "Login" [ref=e1]"
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "- ")
		parts := strings.SplitN(trimmed, " ", 2)
		role := ""
		if len(parts) > 0 {
			role = parts[0]
		}
		refs[ref] = RoleRef{Role: role}
		if IsInteractive(role) {
			interactive++
		}
	}
	return refs, interactive
}

// Screenshot captures a page screenshot via PinchTab.
func (p *PinchTabManager) Screenshot(ctx context.Context, targetID string, fullPage bool) ([]byte, error) {
	path := "/tabs/" + targetID + "/screenshot"
	if fullPage {
		path += "?fullPage=true"
	}
	data, err := p.doGetRaw(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("pinchtab screenshot: %w", err)
	}
	return data, nil
}

// Navigate navigates an existing tab to a new URL.
func (p *PinchTabManager) Navigate(ctx context.Context, targetID string, url string) error {
	_, err := p.doPost(ctx, "/tabs/"+targetID+"/navigate", map[string]any{"url": url})
	return err
}

// ── Actions ───────────────────────────────────────────────────────────────────

func (p *PinchTabManager) Click(ctx context.Context, targetID string, ref string, opts ClickOpts) error {
	body := map[string]any{"kind": "click", "ref": ref}
	if opts.DoubleClick {
		body["doubleClick"] = true
	}
	if opts.Button != "" {
		body["button"] = opts.Button
	}
	_, err := p.doPost(ctx, "/tabs/"+targetID+"/action", body)
	return err
}

func (p *PinchTabManager) Type(ctx context.Context, targetID string, ref string, text string, opts TypeOpts) error {
	body := map[string]any{"kind": "type", "ref": ref, "text": text}
	if opts.Submit {
		body["submit"] = true
	}
	if opts.Slowly {
		body["slowly"] = true
	}
	_, err := p.doPost(ctx, "/tabs/"+targetID+"/action", body)
	return err
}

func (p *PinchTabManager) Press(ctx context.Context, targetID string, key string) error {
	_, err := p.doPost(ctx, "/tabs/"+targetID+"/action", map[string]any{"kind": "press", "key": key})
	return err
}

func (p *PinchTabManager) Hover(ctx context.Context, targetID string, ref string) error {
	_, err := p.doPost(ctx, "/tabs/"+targetID+"/action", map[string]any{"kind": "hover", "ref": ref})
	return err
}

func (p *PinchTabManager) Wait(ctx context.Context, targetID string, opts WaitOpts) error {
	body := map[string]any{"kind": "wait"}
	if opts.TimeMs > 0 {
		body["timeMs"] = opts.TimeMs
	}
	if opts.Text != "" {
		body["text"] = opts.Text
	}
	if opts.TextGone != "" {
		body["textGone"] = opts.TextGone
	}
	if opts.URL != "" {
		body["url"] = opts.URL
	}
	if opts.Fn != "" {
		body["fn"] = opts.Fn
	}
	_, err := p.doPost(ctx, "/tabs/"+targetID+"/action", body)
	return err
}

func (p *PinchTabManager) Evaluate(ctx context.Context, targetID string, fn string) (string, error) {
	data, err := p.doPost(ctx, "/tabs/"+targetID+"/action", map[string]any{"kind": "evaluate", "fn": fn})
	if err != nil {
		return "", err
	}
	var resp ptActionResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return string(data), nil
	}
	return resp.Result, nil
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func (p *PinchTabManager) doGet(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return p.do(req)
}

func (p *PinchTabManager) doGetRaw(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pinchtab HTTP %d: %s", resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}

func (p *PinchTabManager) doPost(ctx context.Context, path string, body map[string]any) ([]byte, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return p.do(req)
}

func (p *PinchTabManager) doDelete(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, p.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return p.do(req)
}

func (p *PinchTabManager) do(req *http.Request) ([]byte, error) {
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pinchtab request %s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("pinchtab HTTP %d %s: %s", resp.StatusCode, req.URL.Path, string(body))
	}
	return body, nil
}

func (p *PinchTabManager) getInstanceLocked(id string) (*ptInstance, error) {
	data, err := p.doGet(context.Background(), "/instances/"+id)
	if err != nil {
		return nil, err
	}
	var inst ptInstance
	if err := json.Unmarshal(data, &inst); err != nil {
		return nil, err
	}
	return &inst, nil
}
