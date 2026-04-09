package browser

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestPinchTabStopUsesStopEndpoints(t *testing.T) {
	var mu sync.Mutex
	var got []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	p := NewPinchTabManager(srv.URL, "", 0)
	p.instanceID = "inst_abc"
	p.profileID = "prof_xyz"

	if err := p.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	want := []string{
		"POST /instances/inst_abc/stop",
		"POST /profiles/prof_xyz/stop",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d requests, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("request %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPinchTabCleanupBrokenStateUsesStopEndpoints(t *testing.T) {
	var mu sync.Mutex
	var got []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	p := NewPinchTabManager(srv.URL, "", 0)
	p.instanceID = "inst_broken"
	p.profileID = "prof_broken"

	p.cleanupBrokenStateLocked(context.Background())

	mu.Lock()
	defer mu.Unlock()

	want := []string{
		"POST /instances/inst_broken/stop",
		"POST /profiles/prof_broken/stop",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d requests, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("request %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPinchTabActionTimeoutConfigurable(t *testing.T) {
	p := NewPinchTabManager("http://example.test", "", 90*time.Second)
	if got := p.ActionTimeout(); got != 90*time.Second {
		t.Fatalf("ActionTimeout() = %v, want %v", got, 90*time.Second)
	}

	p = NewPinchTabManager("http://example.test", "", 0)
	if got := p.ActionTimeout(); got != 120*time.Second {
		t.Fatalf("default ActionTimeout() = %v, want %v", got, 120*time.Second)
	}
}

func TestPinchTabOpenTabFallsBackViaBlankNavigateOnTimeout(t *testing.T) {
	openCalls := 0
	navigateCalls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/instances/inst-1/tabs/open":
			openCalls++
			if openCalls == 1 {
				http.Error(w, "context deadline exceeded", http.StatusGatewayTimeout)
				return
			}
			_ = json.NewEncoder(w).Encode(ptTabOpenResp{TabID: "tab-1", URL: "about:blank", Title: "Blank"})
		case r.Method == http.MethodPost && r.URL.Path == "/tabs/tab-1/navigate":
			navigateCalls++
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	p := NewPinchTabManager(srv.URL, "", 90*time.Second)
	p.instanceID = "inst-1"

	tab, err := p.OpenTab(context.Background(), "https://www.ukr.net")
	if err != nil {
		t.Fatalf("OpenTab() error: %v", err)
	}
	if tab.TargetID != "tab-1" {
		t.Fatalf("TargetID = %q, want %q", tab.TargetID, "tab-1")
	}
	if openCalls != 2 {
		t.Fatalf("open calls = %d, want %d", openCalls, 2)
	}
	if navigateCalls != 1 {
		t.Fatalf("navigate calls = %d, want %d", navigateCalls, 1)
	}
}

func TestShouldFallbackOpenOnEOF(t *testing.T) {
	if !shouldFallbackOpen(errors.New("Post \"http://127.0.0.1:9867/instances/x/tabs/open\": EOF"), "https://www.ukr.net") {
		t.Fatal("expected EOF to trigger fallback")
	}
}
