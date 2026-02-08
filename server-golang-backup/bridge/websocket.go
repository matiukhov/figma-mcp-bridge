package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type Bridge struct {
	addr      string
	upgrader  websocket.Upgrader
	connMu    sync.RWMutex
	conn      *websocket.Conn
	pending   map[string]chan Response
	pendingMu sync.Mutex
	counter   uint64
	mux       *http.ServeMux
	server    *http.Server
}

func NewBridge(addr string) *Bridge {
	return &Bridge{
		addr: addr,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		pending: make(map[string]chan Response),
		mux:     http.NewServeMux(),
	}
}

// Mux returns the HTTP mux so additional handlers can be registered
func (b *Bridge) Mux() *http.ServeMux {
	return b.mux
}

// Start starts the bridge WebSocket server (blocking)
func (b *Bridge) Start() error {
	b.mux.HandleFunc("/ws", b.HandleWebSocket)
	b.server = &http.Server{
		Addr:              b.addr,
		Handler:           b.mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("Bridge listening on %s", b.addr)
	err := b.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Stop gracefully stops the bridge server
func (b *Bridge) Stop() error {
	if b.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return b.server.Shutdown(ctx)
}

func (b *Bridge) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	b.setConn(conn)
	go b.readLoop(conn)
}

func (b *Bridge) setConn(conn *websocket.Conn) {
	b.connMu.Lock()
	defer b.connMu.Unlock()
	if b.conn != nil {
		_ = b.conn.Close()
	}
	b.conn = conn
}

func (b *Bridge) readLoop(conn *websocket.Conn) {
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			b.clearConn(conn)
			return
		}
		var resp Response
		if err := json.Unmarshal(payload, &resp); err != nil {
			log.Printf("Invalid response: %v", err)
			continue
		}
		b.pendingMu.Lock()
		ch := b.pending[resp.RequestID]
		if ch != nil {
			delete(b.pending, resp.RequestID)
		}
		b.pendingMu.Unlock()
		if ch != nil {
			ch <- resp
		}
	}
}

func (b *Bridge) clearConn(conn *websocket.Conn) {
	b.connMu.Lock()
	if b.conn == conn {
		b.conn = nil
	}
	b.connMu.Unlock()
}

func (b *Bridge) Send(ctx context.Context, requestType string, nodeIDs []string) (Response, error) {
	return b.SendWithParams(ctx, requestType, nodeIDs, nil)
}

func (b *Bridge) SendWithParams(ctx context.Context, requestType string, nodeIDs []string, params map[string]interface{}) (Response, error) {
	conn := b.getConn()
	if conn == nil {
		return Response{}, errors.New("plugin not connected")
	}

	requestID := b.nextID()
	req := Request{
		Type:      requestType,
		RequestID: requestID,
		NodeIDs:   nodeIDs,
		Params:    params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}

	respCh := make(chan Response, 1)
	b.pendingMu.Lock()
	b.pending[requestID] = respCh
	b.pendingMu.Unlock()

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		b.pendingMu.Lock()
		delete(b.pending, requestID)
		b.pendingMu.Unlock()
		return Response{}, err
	}

	select {
	case resp := <-respCh:
		if resp.Error != "" {
			return resp, errors.New(resp.Error)
		}
		return resp, nil
	case <-ctx.Done():
		b.pendingMu.Lock()
		delete(b.pending, requestID)
		b.pendingMu.Unlock()
		return Response{}, ctx.Err()
	}
}

func (b *Bridge) getConn() *websocket.Conn {
	b.connMu.RLock()
	defer b.connMu.RUnlock()
	return b.conn
}

func (b *Bridge) nextID() string {
	id := atomic.AddUint64(&b.counter, 1)
	return "req-" + time.Now().Format("150405") + "-" + fmtUint(id)
}

func fmtUint(value uint64) string {
	return strconv.FormatUint(value, 10)
}
