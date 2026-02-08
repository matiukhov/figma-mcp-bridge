package follower

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"figma-mcp-bridge-v2/bridge"
)

// RPCRequest is the format for outgoing RPC requests to the leader
type RPCRequest struct {
	Tool    string                 `json:"tool"`
	NodeIDs []string               `json:"nodeIds,omitempty"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// RPCResponse is the format for RPC responses from the leader
type RPCResponse struct {
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

// Follower proxies MCP tool calls to the leader via HTTP
type Follower struct {
	leaderURL string
	client    *http.Client
}

// New creates a new Follower instance
func New(leaderURL string) *Follower {
	return &Follower{
		leaderURL: leaderURL,
		client: &http.Client{
			Timeout: 35 * time.Second, // Slightly longer than leader's timeout
		},
	}
}

// Send proxies a request to the leader
func (f *Follower) Send(ctx context.Context, requestType string, nodeIDs []string) (bridge.Response, error) {
	return f.SendWithParams(ctx, requestType, nodeIDs, nil)
}

// SendWithParams proxies a request with parameters to the leader
func (f *Follower) SendWithParams(ctx context.Context, requestType string, nodeIDs []string, params map[string]interface{}) (bridge.Response, error) {
	rpcReq := RPCRequest{
		Tool:    requestType,
		NodeIDs: nodeIDs,
		Params:  params,
	}

	body, err := json.Marshal(rpcReq)
	if err != nil {
		return bridge.Response{}, fmt.Errorf("failed to marshal RPC request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.leaderURL+"/rpc", bytes.NewReader(body))
	if err != nil {
		return bridge.Response{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return bridge.Response{}, fmt.Errorf("failed to call leader: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return bridge.Response{}, fmt.Errorf("leader returned status %d", resp.StatusCode)
	}

	var rpcResp RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return bridge.Response{}, fmt.Errorf("failed to decode response: %w", err)
	}

	if rpcResp.Error != "" {
		return bridge.Response{Error: rpcResp.Error}, errors.New(rpcResp.Error)
	}

	// Unmarshal the raw JSON data into interface{}
	var data interface{}
	if len(rpcResp.Data) > 0 {
		if err := json.Unmarshal(rpcResp.Data, &data); err != nil {
			return bridge.Response{}, fmt.Errorf("failed to unmarshal data: %w", err)
		}
	}

	return bridge.Response{
		Type: requestType,
		Data: data,
	}, nil
}

// Ping checks if the leader is reachable and healthy
func (f *Follower) Ping(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.leaderURL+"/ping", nil)
	if err != nil {
		return false
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
