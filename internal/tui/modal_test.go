package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareAdd_AcceptsMagnetURI(t *testing.T) {
	src, _, vErr := prepareAdd("magnet:?xt=urn:btih:abc", "/tmp")
	if vErr != "" {
		t.Fatalf("unexpected validation error: %q", vErr)
	}
	if src != "magnet:?xt=urn:btih:abc" {
		t.Errorf("magnet URI was modified: %q", src)
	}
}

func TestPrepareAdd_AcceptsHTTPSURL(t *testing.T) {
	src, _, vErr := prepareAdd("https://releases.ubuntu.com/foo.torrent", "/tmp")
	if vErr != "" {
		t.Fatalf("unexpected validation error: %q", vErr)
	}
	if src != "https://releases.ubuntu.com/foo.torrent" {
		t.Errorf("URL was modified: %q", src)
	}
}

func TestPrepareAdd_AcceptsHTTPURL(t *testing.T) {
	_, _, vErr := prepareAdd("http://example.com/x.torrent", "/tmp")
	if vErr != "" {
		t.Errorf("unexpected validation error: %q", vErr)
	}
}

func TestPrepareAdd_AcceptsLocalTorrentFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.torrent")
	if err := os.WriteFile(path, []byte("d8:announce..."), 0o644); err != nil {
		t.Fatal(err)
	}
	src, _, vErr := prepareAdd(path, "/tmp")
	if vErr != "" {
		t.Fatalf("unexpected validation error: %q", vErr)
	}
	if src != path {
		t.Errorf("source = %q; want %q", src, path)
	}
}

func TestPrepareAdd_RejectsNonExistentTorrentFile(t *testing.T) {
	_, _, vErr := prepareAdd("/nonexistent/path/file.torrent", "/tmp")
	if vErr == "" {
		t.Errorf("expected validation error for missing file")
	}
	if !strings.Contains(vErr, "not found") {
		t.Errorf("error should mention 'not found', got %q", vErr)
	}
}

func TestPrepareAdd_RejectsFileWithoutTorrentExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, vErr := prepareAdd(path, "/tmp")
	if vErr == "" {
		t.Errorf("expected validation error for non-.torrent file")
	}
}

func TestPrepareAdd_ExpandsTildeInTorrentPath(t *testing.T) {
	// We can't easily create a file under ~ in a test, so verify that the
	// rejection path mentions the expanded (non-tilde) path.
	_, _, vErr := prepareAdd("~/nonexistent-test-file-xyz.torrent", "/tmp")
	if vErr == "" {
		t.Fatal("expected error for nonexistent file under home")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("home dir unavailable: %v", err)
	}
	if strings.Contains(vErr, "~") {
		t.Errorf("error should not contain literal tilde, got %q", vErr)
	}
	if !strings.Contains(vErr, home) {
		t.Errorf("error should mention expanded home path %q, got %q", home, vErr)
	}
}

func TestPrepareAdd_RejectsEmptyInput(t *testing.T) {
	_, _, vErr := prepareAdd("", "/tmp")
	if vErr == "" {
		t.Errorf("expected validation error for empty source")
	}
}

func TestPrepareAdd_RejectsUnknownInput(t *testing.T) {
	_, _, vErr := prepareAdd("not a magnet, URL, or file", "/tmp")
	if vErr == "" {
		t.Errorf("expected validation error for bogus input")
	}
}

func TestPrepareAdd_TrimsWhitespaceAroundInputs(t *testing.T) {
	src, saveDir, vErr := prepareAdd("   magnet:?xt=urn:btih:abc   ", "  /tmp/dl  ")
	if vErr != "" {
		t.Fatalf("unexpected validation error: %q", vErr)
	}
	if src != "magnet:?xt=urn:btih:abc" {
		t.Errorf("source not trimmed: %q", src)
	}
	if saveDir != "/tmp/dl" {
		t.Errorf("saveDir not trimmed: %q", saveDir)
	}
}

func TestPrepareAdd_ExpandsTildeInSaveDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("home dir unavailable: %v", err)
	}
	_, saveDir, vErr := prepareAdd("magnet:?xt=urn:btih:abc", "~/Downloads/test")
	if vErr != "" {
		t.Fatalf("unexpected validation error: %q", vErr)
	}
	want := filepath.Join(home, "Downloads", "test")
	if saveDir != want {
		t.Errorf("saveDir = %q; want %q", saveDir, want)
	}
}

func TestPrepareAdd_EmptySaveDirAllowed(t *testing.T) {
	_, saveDir, vErr := prepareAdd("magnet:?xt=urn:btih:abc", "")
	if vErr != "" {
		t.Errorf("empty saveDir should not be a validation error: %q", vErr)
	}
	if saveDir != "" {
		t.Errorf("expected empty saveDir to pass through, got %q", saveDir)
	}
}

func TestIsAddableSource(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"magnet URI", "magnet:?xt=urn:btih:abc", true},
		{"magnet with whitespace", "  magnet:?xt=urn:btih:abc  ", true},
		{".torrent over https", "https://example.com/foo.torrent", true},
		{".torrent over http", "http://example.com/foo.torrent", true},
		{".torrent URL with query string", "https://example.com/foo.torrent?key=abc", true},
		{"bare URL is NOT auto-pastable", "https://example.com/", false},
		{"non-.torrent URL", "https://news.ycombinator.com/", false},
		{"random text", "hello world", false},
		{"empty string", "", false},
		{"local file path is NOT auto-pasted (uncommon clipboard)", "/Users/x/foo.torrent", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAddableSource(tc.input); got != tc.want {
				t.Errorf("isAddableSource(%q) = %v; want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestCleanError_StripsRPCPrefix(t *testing.T) {
	got := cleanError(errors.New("RPC: download directory not accessible"))
	want := "download directory not accessible"
	if got != want {
		t.Errorf("cleanError = %q; want %q", got, want)
	}
}

func TestCleanError_LeavesOtherErrorsAlone(t *testing.T) {
	got := cleanError(errors.New("connection refused"))
	if got != "connection refused" {
		t.Errorf("non-RPC error should pass through, got %q", got)
	}
}

func TestCleanError_NilReturnsEmpty(t *testing.T) {
	if got := cleanError(nil); got != "" {
		t.Errorf("cleanError(nil) = %q; want empty", got)
	}
}
