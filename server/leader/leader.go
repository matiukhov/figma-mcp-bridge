package leader

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"figma-mcp-bridge-v2/bridge"
)

// RPCRequest is the format for incoming RPC requests from followers
type RPCRequest struct {
	Tool    string                 `json:"tool"`
	NodeIDs []string               `json:"nodeIds,omitempty"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// RPCResponse is the format for RPC responses to followers
type RPCResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// Leader owns the WebSocket bridge to Figma and exposes HTTP endpoints for followers
type Leader struct {
	addr     string
	bridge   *bridge.Bridge
	listener net.Listener
	server   *http.Server
	wg       sync.WaitGroup
}

// New creates a new Leader instance
func New(addr string) *Leader {
	return &Leader{
		addr: addr,
	}
}

// Bridge returns the underlying bridge for direct access
func (l *Leader) Bridge() *bridge.Bridge {
	return l.bridge
}

// Start starts the leader with bridge and HTTP endpoints
func (l *Leader) Start() error {
	// Try to bind the port first to fail fast if already taken
	listener, err := net.Listen("tcp", l.addr)
	if err != nil {
		return err // Port already in use
	}
	l.listener = listener

	// Create the bridge with the same address
	l.bridge = bridge.NewBridge(l.addr)

	// Get the mux and add our HTTP endpoints
	mux := l.bridge.Mux()
	mux.HandleFunc("/ping", l.handlePing)
	mux.HandleFunc("/rpc", l.handleRPC)
	mux.HandleFunc("/ws", l.bridge.HandleWebSocket)

	// Create server with the bridge's mux
	l.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start serving on the listener we already bound
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		log.Printf("Leader listening on %s", l.addr)
		if err := l.server.Serve(l.listener); err != nil && err != http.ErrServerClosed {
			log.Printf("Leader server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully stops the leader
func (l *Leader) Stop() {
	if l.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := l.server.Shutdown(ctx); err != nil {
			log.Printf("Leader shutdown error: %v", err)
		}
	}
	l.wg.Wait()
}

// handlePing responds to health check requests
func (l *Leader) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"version": "0.1.0",
	})
}

// handleRPC handles RPC requests from followers
func (l *Leader) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(RPCResponse{Error: "invalid request body"})
		return
	}

	// Forward to bridge with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	resp, err := l.bridge.SendWithParams(ctx, req.Tool, req.NodeIDs, req.Params)

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		json.NewEncoder(w).Encode(RPCResponse{Error: err.Error()})
		return
	}

	json.NewEncoder(w).Encode(RPCResponse{Data: resp.Data})
}
