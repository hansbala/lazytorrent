package transmission

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeServer returns the canonical Transmission 409-then-200 handshake.
// First request without X-Transmission-Session-Id returns 409 + header.
// Subsequent requests with the header return the supplied body.
func fakeServer(t *testing.T, sessionID, body string) (*httptest.Server, *int) {
	t.Helper()
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Header.Get("X-Transmission-Session-Id") == "" {
			w.Header().Set("X-Transmission-Session-Id", sessionID)
			w.WriteHeader(http.StatusConflict)
			return
		}
		if r.Header.Get("X-Transmission-Session-Id") != sessionID {
			t.Errorf("expected session ID %q, got %q", sessionID, r.Header.Get("X-Transmission-Session-Id"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	return srv, &attempts
}

func TestSessionGet_PerformsSessionIDHandshakeOn409(t *testing.T) {
	srv, attempts := fakeServer(t, "test-session-id",
		`{"result":"success","arguments":{"version":"4.1.1","rpc-version":18,"download-dir":"/tmp"}}`)
	defer srv.Close()

	c := New(srv.URL)
	info, err := c.SessionGet()
	if err != nil {
		t.Fatalf("SessionGet: %v", err)
	}
	if info.Version != "4.1.1" {
		t.Errorf("Version: got %q want 4.1.1", info.Version)
	}
	if info.RPCVersion != 18 {
		t.Errorf("RPCVersion: got %d want 18", info.RPCVersion)
	}
	if info.DownloadDir != "/tmp" {
		t.Errorf("DownloadDir: got %q want /tmp", info.DownloadDir)
	}
	if *attempts != 2 {
		t.Errorf("expected 2 HTTP attempts (409 + 200), got %d", *attempts)
	}
}

func TestSessionGet_SubsequentCallsReuseSessionID(t *testing.T) {
	srv, attempts := fakeServer(t, "stable-id",
		`{"result":"success","arguments":{"version":"4.1.1","rpc-version":18,"download-dir":"/tmp"}}`)
	defer srv.Close()

	c := New(srv.URL)
	if _, err := c.SessionGet(); err != nil {
		t.Fatalf("first SessionGet: %v", err)
	}
	if _, err := c.SessionGet(); err != nil {
		t.Fatalf("second SessionGet: %v", err)
	}
	// First call: 2 attempts (409 + 200). Second call should use cached ID: 1 attempt.
	if *attempts != 3 {
		t.Errorf("expected 3 total attempts (2 for first call, 1 for second), got %d", *attempts)
	}
}

func TestCall_Returns401WhenAuthRequired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.SessionGet()
	if err == nil {
		t.Fatalf("expected an error on 401")
	}
	if !strings.Contains(err.Error(), "auth") {
		t.Errorf("expected error to mention auth, got %q", err)
	}
}

func TestCall_PropagatesRPCResultError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Transmission-Session-Id") == "" {
			w.Header().Set("X-Transmission-Session-Id", "x")
			w.WriteHeader(http.StatusConflict)
			return
		}
		_, _ = w.Write([]byte(`{"result":"invalid argument","arguments":{}}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.SessionGet()
	if err == nil || !strings.Contains(err.Error(), "invalid argument") {
		t.Errorf("expected RPC error to surface 'invalid argument', got %v", err)
	}
}
