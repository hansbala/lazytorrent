package transmission

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const DefaultURL = "http://127.0.0.1:9091/transmission/rpc"

type Client struct {
	URL       string
	User      string
	Pass      string
	sessionID string
	http      *http.Client
}

func New(url string) *Client {
	return &Client{
		URL:  url,
		http: &http.Client{Timeout: 5 * time.Second},
	}
}

type rpcRequest struct {
	Method    string `json:"method"`
	Arguments any    `json:"arguments,omitempty"`
}

type rpcResponse struct {
	Result    string          `json:"result"`
	Arguments json.RawMessage `json:"arguments"`
}

func (c *Client) Call(method string, args any, out any) error {
	body, err := json.Marshal(rpcRequest{Method: method, Arguments: args})
	if err != nil {
		return err
	}

	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequest("POST", c.URL, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		if c.sessionID != "" {
			req.Header.Set("X-Transmission-Session-Id", c.sessionID)
		}
		if c.User != "" {
			req.SetBasicAuth(c.User, c.Pass)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusConflict {
			c.sessionID = resp.Header.Get("X-Transmission-Session-Id")
			resp.Body.Close()
			continue
		}
		if resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()
			return fmt.Errorf("RPC auth required (HTTP 401)")
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("RPC HTTP %d", resp.StatusCode)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}

		var r rpcResponse
		if err := json.Unmarshal(respBody, &r); err != nil {
			return err
		}
		if r.Result != "success" {
			return fmt.Errorf("RPC: %s", r.Result)
		}
		if out != nil && len(r.Arguments) > 0 {
			return json.Unmarshal(r.Arguments, out)
		}
		return nil
	}
	return fmt.Errorf("RPC session handshake failed")
}

type SessionInfo struct {
	Version     string `json:"version"`
	DownloadDir string `json:"download-dir"`
	RPCVersion  int    `json:"rpc-version"`
}

func (c *Client) SessionGet() (*SessionInfo, error) {
	var info SessionInfo
	if err := c.Call("session-get", nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// Transmission status codes (from libtransmission).
const (
	StatusStopped      = 0
	StatusCheckWait    = 1
	StatusCheck        = 2
	StatusDownloadWait = 3
	StatusDownload     = 4
	StatusSeedWait     = 5
	StatusSeed         = 6
)

func StatusString(s int) string {
	switch s {
	case StatusStopped:
		return "Stopped"
	case StatusCheckWait, StatusCheck:
		return "Verifying"
	case StatusDownloadWait:
		return "Queued"
	case StatusDownload:
		return "Downloading"
	case StatusSeedWait:
		return "Queued"
	case StatusSeed:
		return "Seeding"
	}
	return "Unknown"
}

type Torrent struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Status         int     `json:"status"`
	PercentDone    float64 `json:"percentDone"`
	RateDownload   int64   `json:"rateDownload"`
	RateUpload     int64   `json:"rateUpload"`
	ETA            int64   `json:"eta"`
	TotalSize      int64   `json:"totalSize"`
	DownloadedEver int64   `json:"downloadedEver"`
	UploadedEver   int64   `json:"uploadedEver"`
	UploadRatio    float64 `json:"uploadRatio"`
	DownloadDir    string  `json:"downloadDir"`
	AddedDate      int64   `json:"addedDate"`
	PeersConnected int     `json:"peersConnected"`
}

var torrentFields = []string{
	"id", "name", "status", "percentDone",
	"rateDownload", "rateUpload", "eta",
	"totalSize", "downloadedEver", "uploadedEver",
	"uploadRatio", "downloadDir", "addedDate",
	"peersConnected",
}

func (c *Client) TorrentGet() ([]Torrent, error) {
	args := map[string]any{"fields": torrentFields}
	var resp struct {
		Torrents []Torrent `json:"torrents"`
	}
	if err := c.Call("torrent-get", args, &resp); err != nil {
		return nil, err
	}
	return resp.Torrents, nil
}

type AddResult struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	HashString string `json:"hashString"`
	Duplicate  bool   `json:"-"`
}

// TorrentAdd queues a torrent on the daemon. `filename` may be a magnet URI,
// the path to a .torrent file the daemon can read, or base64-encoded torrent data.
// `downloadDir` may be empty to use the daemon's default.
// Returns Duplicate=true if Transmission already has this torrent.
func (c *Client) TorrentAdd(filename, downloadDir string) (*AddResult, error) {
	args := map[string]any{
		"filename": filename,
		"paused":   false,
	}
	if downloadDir != "" {
		args["download-dir"] = downloadDir
	}
	var resp struct {
		TorrentAdded     *AddResult `json:"torrent-added"`
		TorrentDuplicate *AddResult `json:"torrent-duplicate"`
	}
	if err := c.Call("torrent-add", args, &resp); err != nil {
		return nil, err
	}
	if resp.TorrentDuplicate != nil {
		resp.TorrentDuplicate.Duplicate = true
		return resp.TorrentDuplicate, nil
	}
	if resp.TorrentAdded != nil {
		return resp.TorrentAdded, nil
	}
	return nil, fmt.Errorf("torrent-add: empty response")
}
