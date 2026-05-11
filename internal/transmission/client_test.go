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

func TestTorrentGet_DecodesTorrentArray(t *testing.T) {
	body := `{"result":"success","arguments":{"torrents":[
		{"id":1,"name":"ubuntu.iso","status":4,"percentDone":0.87,"rateDownload":4400000,"rateUpload":312000,"eta":134,"totalSize":4194304000,"downloadedEver":3650000000,"uploadedEver":0,"uploadRatio":0,"downloadDir":"/Users/x/Downloads","addedDate":1715000000,"peersConnected":42},
		{"id":2,"name":"debian.iso","status":6,"percentDone":1,"rateDownload":0,"rateUpload":22000,"eta":-1,"totalSize":700000000,"downloadedEver":700000000,"uploadedEver":938000000,"uploadRatio":1.34,"downloadDir":"/Users/x/Downloads","addedDate":1714000000,"peersConnected":12}
	]}}`
	srv, _ := fakeServer(t, "sid", body)
	defer srv.Close()

	c := New(srv.URL)
	ts, err := c.TorrentGet()
	if err != nil {
		t.Fatalf("TorrentGet: %v", err)
	}
	if len(ts) != 2 {
		t.Fatalf("expected 2 torrents, got %d", len(ts))
	}
	if ts[0].Name != "ubuntu.iso" || ts[0].PercentDone != 0.87 || ts[0].Status != StatusDownload {
		t.Errorf("torrent 0 decoded incorrectly: %+v", ts[0])
	}
	if ts[1].UploadRatio != 1.34 || ts[1].Status != StatusSeed {
		t.Errorf("torrent 1 decoded incorrectly: %+v", ts[1])
	}
}

func TestTorrentAdd_NewTorrent(t *testing.T) {
	body := `{"result":"success","arguments":{"torrent-added":{"id":7,"name":"ubuntu.iso","hashString":"abc123"}}}`
	srv, _ := fakeServer(t, "sid", body)
	defer srv.Close()

	c := New(srv.URL)
	r, err := c.TorrentAdd("magnet:?xt=urn:btih:abc", "/tmp/dl")
	if err != nil {
		t.Fatalf("TorrentAdd: %v", err)
	}
	if r.ID != 7 || r.Name != "ubuntu.iso" || r.Duplicate {
		t.Errorf("got %+v", r)
	}
}

func TestTorrentAdd_Duplicate(t *testing.T) {
	body := `{"result":"success","arguments":{"torrent-duplicate":{"id":7,"name":"ubuntu.iso","hashString":"abc123"}}}`
	srv, _ := fakeServer(t, "sid", body)
	defer srv.Close()

	c := New(srv.URL)
	r, err := c.TorrentAdd("magnet:?xt=urn:btih:abc", "")
	if err != nil {
		t.Fatalf("TorrentAdd returned err on duplicate: %v", err)
	}
	if !r.Duplicate {
		t.Errorf("expected Duplicate=true, got %+v", r)
	}
}

func TestStatusString_CoversAllCodes(t *testing.T) {
	cases := map[int]string{
		StatusStopped:      "Stopped",
		StatusCheckWait:    "Verifying",
		StatusCheck:        "Verifying",
		StatusDownloadWait: "Queued",
		StatusDownload:     "Downloading",
		StatusSeedWait:     "Queued",
		StatusSeed:         "Seeding",
		999:                "Unknown",
	}
	for code, want := range cases {
		if got := StatusString(code); got != want {
			t.Errorf("StatusString(%d) = %q; want %q", code, got, want)
		}
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
