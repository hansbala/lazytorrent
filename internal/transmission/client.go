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
