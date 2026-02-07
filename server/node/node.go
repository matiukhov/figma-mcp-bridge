package node

import (
	"context"
	"log"
	"sync"

	"figma-mcp-bridge-v2/bridge"
	"figma-mcp-bridge-v2/follower"
	"figma-mcp-bridge-v2/leader"
)

// Role represents the current role of this node in the cluster
type Role int

const (
	RoleUnknown Role = iota
	RoleLeader
	RoleFollower
)

func (r Role) String() string {
	switch r {
	case RoleLeader:
		return "LEADER"
	case RoleFollower:
		return "FOLLOWER"
	default:
		return "UNKNOWN"
	}
}

// Node is the dynamic handler that switches between leader and follower roles.
// It implements the ToolHandler interface used by MCP tools.
type Node struct {
	mu       sync.RWMutex
	role     Role
	leader   *leader.Leader
	follower *follower.Follower
	addr     string
}

// New creates a new Node instance
func New(addr string) *Node {
	return &Node{
		addr:     addr,
		follower: follower.New("http://localhost:1994"),
	}
}

// Role returns the current role of this node
func (n *Node) Role() Role {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.role
}

// RoleInt returns the current role as int (for election package interface)
func (n *Node) RoleInt() int {
	return int(n.Role())
}

// Send implements ToolHandler - routes request based on current role
func (n *Node) Send(ctx context.Context, requestType string, nodeIDs []string) (bridge.Response, error) {
	return n.SendWithParams(ctx, requestType, nodeIDs, nil)
}

// SendWithParams implements ToolHandler - routes request based on current role
func (n *Node) SendWithParams(ctx context.Context, requestType string, nodeIDs []string, params map[string]interface{}) (bridge.Response, error) {
	n.mu.RLock()
	role := n.role
	l := n.leader
	f := n.follower
	n.mu.RUnlock()

	// Dynamic dispatch based on CURRENT role
	if role == RoleLeader && l != nil {
		return l.Bridge().SendWithParams(ctx, requestType, nodeIDs, params)
	}
	return f.SendWithParams(ctx, requestType, nodeIDs, params)
}

// BecomeLeader attempts to transition this node to the leader role
func (n *Node) BecomeLeader() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.role == RoleLeader {
		return nil // already leader
	}

	// Start the leader (bridge + HTTP server)
	l := leader.New(n.addr)
	if err := l.Start(); err != nil {
		return err
	}

	n.leader = l
	n.role = RoleLeader
	log.Println("Became LEADER")
	return nil
}

// BecomeFollower transitions this node to the follower role
func (n *Node) BecomeFollower() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.role == RoleFollower {
		return
	}

	// Stop leader if we were one
	if n.leader != nil {
		n.leader.Stop()
		n.leader = nil
	}

	n.role = RoleFollower
	log.Println("Became FOLLOWER")
}

// Stop gracefully stops the node
func (n *Node) Stop() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.leader != nil {
		n.leader.Stop()
		n.leader = nil
	}
	n.role = RoleUnknown
}
