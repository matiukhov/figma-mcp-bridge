package election

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"time"
)

// RoleChanger is implemented by Node to allow election to trigger role changes
type RoleChanger interface {
	BecomeLeader() error
	BecomeFollower()
	RoleInt() int // Returns node.Role as int to avoid circular import
}

// Role constants (must match node.Role values)
const (
	RoleUnknown = iota
	RoleLeader
	RoleFollower
)

// Election handles leader detection and role transitions
type Election struct {
	node       RoleChanger
	addr       string
	leaderURL  string
	ticker     *time.Ticker
	stopCh     chan struct{}
	httpClient *http.Client
}

// New creates a new Election instance
func New(addr string, n RoleChanger) *Election {
	return &Election{
		node:      n,
		addr:      addr,
		leaderURL: "http://localhost:1994",
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

// Start begins the election process and continuous monitoring
func (e *Election) Start() {
	// Random jitter between 3-5 seconds to stagger multiple instances
	jitter := 3 + rand.Intn(3)
	e.ticker = time.NewTicker(time.Duration(jitter) * time.Second)
	e.stopCh = make(chan struct{})

	// Initial role determination
	e.determineRole()

	// Continuous monitoring
	go e.runTicker()
}

// Stop stops the election monitoring
func (e *Election) Stop() {
	if e.ticker != nil {
		e.ticker.Stop()
	}
	if e.stopCh != nil {
		close(e.stopCh)
	}
}

func (e *Election) runTicker() {
	for {
		select {
		case <-e.ticker.C:
			e.checkAndUpdateRole()
		case <-e.stopCh:
			return
		}
	}
}

func (e *Election) checkAndUpdateRole() {
	currentRole := e.node.RoleInt()

	switch currentRole {
	case RoleFollower:
		// Check if leader is still alive
		if !e.pingLeader() {
			// Leader died - try to take over
			log.Println("Leader not responding, attempting takeover...")
			if err := e.node.BecomeLeader(); err != nil {
				log.Printf("Failed to become leader: %v", err)
			}
		}

	case RoleLeader:
		// We are leader, nothing to do
		// Could add self-health checks here if needed

	case RoleUnknown:
		e.determineRole()
	}
}

func (e *Election) determineRole() {
	// Try to become leader first
	if err := e.node.BecomeLeader(); err == nil {
		// Success, we are leader
		return
	}

	// Port taken - check if it's a valid leader
	if e.pingLeader() {
		e.node.BecomeFollower()
	}
	// If ping fails, next tick will retry
}

func (e *Election) pingLeader() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.leaderURL+"/ping", nil)
	if err != nil {
		return false
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
