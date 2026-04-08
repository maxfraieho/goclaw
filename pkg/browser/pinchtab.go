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
	baseURL    string // e.g. "http://localhost:9867"
	token      string // Bearer token for Authorization header (optional)
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
	ID        string `json:"id"`
	ProfileID string `json:"profileId,omitempty"`
	ProfileName string `json:"profileName,omitempty"`
	Mode      string `json:"mode"`
	Status    string `json:"status,omitempty"`
	Error     string `json:"error,omitempty"`
	LastError string `json:"lastError,omitempty"`
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
			if err := p.waitInstanceReadyLocked(ctx, p.instanceID); err == nil {
				return nil
			}
		}
		p.logger.Info("pinchtab: instance gone, recreating", "instance", p.instanceID)
		p.instanceID = ""
		p.profileID = ""
	}

	if err := p.startWithRecoveryLocked(ctx); err != nil {
		return err
	}
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
	_ = p.stopInstanceLocked(ctx, p.instanceID)
	if p.profileID != "" {
		_ = p.stopProfileLocked(ctx, p.profileID)
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
	tabs, err := decodePinchTabTabs(data)
	if err != nil {
		return nil, fmt.Errorf("pinchtab tabs decode: %w", err)
	}
	result := make([]TabInfo, len(tabs))
	for i, t := range tabs {
		result[i] = TabInfo{TargetID: t.TabID, URL: t.URL, Title: t.Title}
	}
	return result, nil
}

func decodePinchTabTabs(data []byte) ([]ptTabInfo, error) {
	var wrapped ptTabsResp
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Tabs != nil {
		return wrapped.Tabs, nil
	}

	var direct []ptTabInfo
	if err := json.Unmarshal(data, &direct); err == nil {
		return direct, nil
	}

	var wrappedAny map[string]json.RawMessage
	if err := json.Unmarshal(data, &wrappedAny); err == nil {
		if raw, ok := wrappedAny["tabs"]; ok {
			var tabs []ptTabInfo
			if err := json.Unmarshal(raw, &tabs); err == nil {
				return tabs, nil
			}
		}
	}

	return nil, fmt.Errorf("unexpected response: %s", string(data))
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

// ActionTimeout returns the default per-action timeout for PinchTab operations.
func (p *PinchTabManager) ActionTimeout() time.Duration {
	return 30 * time.Second
}

// ConsoleMessages returns captured console messages for a tab.
// PinchTab tracks console output since v0.8 (feat: add console and errors tracking).
func (p *PinchTabManager) ConsoleMessages(ctx context.Context, targetID string) []ConsoleMessage {
	data, err := p.doGet(ctx, "/tabs/"+targetID+"/console")
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

func (p *PinchTabManager) stopInstanceLocked(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	_, err := p.doPost(ctx, "/instances/"+id+"/stop", map[string]any{})
	if err != nil {
		return fmt.Errorf("pinchtab stop instance %s: %w", id, err)
	}
	return nil
}

func (p *PinchTabManager) stopProfileLocked(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	_, err := p.doPost(ctx, "/profiles/"+id+"/stop", map[string]any{})
	if err != nil {
		return fmt.Errorf("pinchtab stop profile %s: %w", id, err)
	}
	return nil
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

func (p *PinchTabManager) startWithRecoveryLocked(ctx context.Context) error {
	var firstErr error
	for attempt := 1; attempt <= 2; attempt++ {
		if attempt > 1 {
			p.logger.Warn("pinchtab: retrying instance startup after cleanup", "attempt", attempt)
			p.cleanupBrokenStateLocked(ctx)
		}
		if err := p.startOnceLocked(ctx); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			p.logger.Warn("pinchtab: startup attempt failed", "attempt", attempt, "error", err)
			continue
		}
		return nil
	}
	return firstErr
}

func (p *PinchTabManager) startOnceLocked(ctx context.Context) error {
	profileID, err := p.ensureProfileLocked(ctx, "goclaw")
	if err != nil {
		return err
	}
	p.profileID = profileID

	inst, err := p.doPost(ctx, "/instances/start", map[string]any{
		"profileId": p.profileID,
		"mode":      "headless",
	})
	if err != nil {
		if strings.Contains(err.Error(), "already has an active instance") {
			instID, findErr := p.findActiveInstanceForProfileLocked(ctx, p.profileID)
			if findErr != nil {
				return fmt.Errorf("pinchtab start instance: %w (active lookup: %v)", err, findErr)
			}
			if instID == "" {
				return fmt.Errorf("pinchtab start instance: %w (active instance not found)", err)
			}
			p.logger.Info("pinchtab: reusing active instance", "instance", instID, "profile", p.profileID)
			p.instanceID = instID
			return p.waitInstanceReadyLocked(ctx, p.instanceID)
		}
		return fmt.Errorf("pinchtab start instance: %w", err)
	}
	var instResp ptInstance
	if err := json.Unmarshal(inst, &instResp); err != nil {
		return fmt.Errorf("pinchtab instance decode: %w", err)
	}
	if instResp.ID == "" {
		return fmt.Errorf("pinchtab start instance: empty instance id")
	}
	p.instanceID = instResp.ID
	if err := p.waitInstanceReadyLocked(ctx, p.instanceID); err != nil {
		return err
	}
	return nil
}

func (p *PinchTabManager) ensureProfileLocked(ctx context.Context, name string) (string, error) {
	prof, err := p.doPost(ctx, "/profiles", map[string]any{"name": name})
	if err != nil {
		if !strings.Contains(err.Error(), "409") {
			return "", fmt.Errorf("pinchtab create profile: %w", err)
		}
		existing, findErr := p.findProfileByNameLocked(ctx, name)
		if findErr != nil {
			return "", fmt.Errorf("pinchtab create profile: %w (list fallback: %v)", err, findErr)
		}
		if existing == "" {
			return "", fmt.Errorf("pinchtab create profile: %w (profile not found after 409)", err)
		}
		p.logger.Info("pinchtab: reusing existing profile", "profile", existing)
		return existing, nil
	}
	var profResp ptProfile
	if err := json.Unmarshal(prof, &profResp); err != nil {
		return "", fmt.Errorf("pinchtab profile decode: %w", err)
	}
	if profResp.ID == "" {
		return "", fmt.Errorf("pinchtab profile decode: empty profile id")
	}
	return profResp.ID, nil
}

func (p *PinchTabManager) findProfileByNameLocked(ctx context.Context, name string) (string, error) {
	all, err := p.doGet(ctx, "/profiles")
	if err != nil {
		return p.findProfileByNameFromInstancesLocked(ctx, name)
	}
	profiles, err := decodePinchTabProfiles(all)
	if err != nil {
		return p.findProfileByNameFromInstancesLocked(ctx, name)
	}
	for _, pp := range profiles {
		if pp.Name == name {
			return pp.ID, nil
		}
	}
	return p.findProfileByNameFromInstancesLocked(ctx, name)
}

func decodePinchTabProfiles(data []byte) ([]ptProfile, error) {
	var direct []ptProfile
	if err := json.Unmarshal(data, &direct); err == nil {
		return direct, nil
	}

	var wrapped map[string]json.RawMessage
	if err := json.Unmarshal(data, &wrapped); err == nil {
		for _, key := range []string{"profiles", "items"} {
			raw, ok := wrapped[key]
			if !ok {
				continue
			}
			var profiles []ptProfile
			if err := json.Unmarshal(raw, &profiles); err == nil {
				return profiles, nil
			}
		}
	}

	return nil, fmt.Errorf("unexpected response: %s", string(data))
}

func (p *PinchTabManager) findProfileByNameFromInstancesLocked(ctx context.Context, name string) (string, error) {
	instances, err := p.listInstancesLocked(ctx)
	if err != nil {
		return "", err
	}
	for _, inst := range instances {
		if inst.ProfileName == name && inst.ProfileID != "" {
			return inst.ProfileID, nil
		}
	}
	return "", nil
}

func (p *PinchTabManager) findActiveInstanceForProfileLocked(ctx context.Context, profileID string) (string, error) {
	instances, err := p.listInstancesLocked(ctx)
	if err != nil {
		return "", err
	}
	for _, inst := range instances {
		if inst.ProfileID != profileID {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(inst.Status))
		if status == "" || status == "running" || status == "starting" || status == "ready" {
			return inst.ID, nil
		}
	}
	return "", nil
}

func (p *PinchTabManager) listInstancesLocked(ctx context.Context) ([]ptInstance, error) {
	data, err := p.doGet(ctx, "/instances")
	if err != nil {
		return nil, err
	}

	var wrapped ptInstancesResp
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Instances != nil {
		return wrapped.Instances, nil
	}

	var direct []ptInstance
	if err := json.Unmarshal(data, &direct); err == nil {
		return direct, nil
	}

	var wrappedAny map[string]json.RawMessage
	if err := json.Unmarshal(data, &wrappedAny); err == nil {
		if raw, ok := wrappedAny["instances"]; ok {
			var instances []ptInstance
			if err := json.Unmarshal(raw, &instances); err == nil {
				return instances, nil
			}
		}
	}

	return nil, fmt.Errorf("pinchtab instances decode: unexpected response: %s", string(data))
}

func (p *PinchTabManager) waitInstanceReadyLocked(ctx context.Context, id string) error {
	deadline := time.Now().Add(12 * time.Second)
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return fmt.Errorf("pinchtab instance %s not ready: %w", id, lastErr)
			}
			return fmt.Errorf("pinchtab instance %s startup canceled: %w", id, err)
		}
		inst, err := p.getInstanceLocked(id)
		if err == nil {
			status := strings.ToLower(strings.TrimSpace(inst.Status))
			if status == "error" || status == "crashed" || status == "failed" {
				msg := strings.TrimSpace(inst.Error)
				if msg == "" {
					msg = strings.TrimSpace(inst.LastError)
				}
				if msg == "" {
					msg = "instance entered error state"
				}
				return fmt.Errorf("pinchtab instance %s unhealthy: %s", id, msg)
			}
			if _, tabsErr := p.listTabsLocked(ctx); tabsErr == nil {
				return nil
			} else {
				lastErr = tabsErr
			}
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return fmt.Errorf("pinchtab instance %s not ready: %w", id, lastErr)
			}
			return fmt.Errorf("pinchtab instance %s not ready before timeout", id)
		}
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("pinchtab instance %s not ready: %w", id, lastErr)
			}
			return fmt.Errorf("pinchtab instance %s startup canceled: %w", id, ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (p *PinchTabManager) cleanupBrokenStateLocked(ctx context.Context) {
	if p.instanceID != "" {
		if err := p.stopInstanceLocked(ctx, p.instanceID); err != nil {
			p.logger.Warn("pinchtab: failed to delete broken instance", "instance", p.instanceID, "error", err)
		}
	}
	if p.profileID != "" {
		if err := p.stopProfileLocked(ctx, p.profileID); err != nil {
			p.logger.Warn("pinchtab: failed to delete broken profile", "profile", p.profileID, "error", err)
		}
	}
	p.instanceID = ""
	p.profileID = ""
}
