package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
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

	p := NewPinchTabManager(srv.URL, "")
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

	p := NewPinchTabManager(srv.URL, "")
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
