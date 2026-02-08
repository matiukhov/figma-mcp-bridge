package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"figma-mcp-bridge-v2/election"
	mcpbridge "figma-mcp-bridge-v2/mcp"
	"figma-mcp-bridge-v2/node"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	addr := ":1994"

	// Create the dynamic node (handles both roles)
	n := node.New(addr)

	// Start election (determines initial role + monitors)
	e := election.New(addr, n)
	e.Start()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		e.Stop()
		n.Stop()
		os.Exit(0)
	}()

	// MCP tools use the Node as handler - it routes dynamically
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "figma-bridge",
		Version: "0.1.0",
	}, nil)

	tools := &mcpbridge.Tools{Handler: n}
	tools.Register(server)

	log.Printf("Starting MCP server (role: %s)", n.Role())
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("MCP server failed: %v", err)
	}
}
