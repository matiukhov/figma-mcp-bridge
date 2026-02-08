package mcpbridge

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"figma-mcp-bridge-v2/bridge"
)

// ToolHandler abstracts the bridge communication.
// Implemented by Node, which delegates to leader.Bridge or follower.Proxy based on current role.
type ToolHandler interface {
	Send(ctx context.Context, requestType string, nodeIDs []string) (bridge.Response, error)
	SendWithParams(ctx context.Context, requestType string, nodeIDs []string, params map[string]interface{}) (bridge.Response, error)
}

type Tools struct {
	Handler ToolHandler
}

func (t *Tools) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_document",
		Description: "Get the current Figma page document tree",
	}, t.handleGetDocument)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_selection",
		Description: "Get the currently selected nodes in Figma",
	}, t.handleGetSelection)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_node",
		Description: "Get a specific Figma node by ID",
	}, t.handleGetNode)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_styles",
		Description: "Get all local styles in the document",
	}, t.handleGetStyles)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_metadata",
		Description: "Get metadata about the current Figma document including file name, pages, and current page info",
	}, t.handleGetMetadata)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_design_context",
		Description: "Get the design context for the current selection or page. Returns a summarized tree structure optimized for understanding the current design context.",
	}, t.handleGetDesignContext)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_variable_defs",
		Description: "Get all local variable definitions including variable collections, modes, and variable values. Variables are Figma's system for design tokens (colors, numbers, strings, booleans).",
	}, t.handleGetVariableDefs)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_screenshot",
		Description: "Export a screenshot of the selected nodes or specific nodes by ID. Returns base64-encoded image data.",
	}, t.handleGetScreenshot)
}

type getNodeArgs struct {
	NodeID string `json:"nodeId" jsonschema:"the node ID to fetch"`
}

type getDesignContextArgs struct {
	Depth int `json:"depth,omitempty" jsonschema:"how many levels deep to traverse the node tree (default 2)"`
}

type getScreenshotArgs struct {
	NodeIDs []string `json:"nodeIds,omitempty" jsonschema:"optional list of node IDs to export - if empty exports the current selection"`
	Format  string   `json:"format,omitempty" jsonschema:"export format: PNG (default) or SVG or JPG or PDF"`
	Scale   float64  `json:"scale,omitempty" jsonschema:"export scale for raster formats (default 2)"`
}

func (t *Tools) handleGetDocument(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ struct{},
) (*mcp.CallToolResult, any, error) {
	resp, err := t.Handler.Send(ctx, "get_document", nil)
	return renderResponse(resp, err)
}

func (t *Tools) handleGetSelection(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ struct{},
) (*mcp.CallToolResult, any, error) {
	resp, err := t.Handler.Send(ctx, "get_selection", nil)
	return renderResponse(resp, err)
}

func (t *Tools) handleGetNode(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	args getNodeArgs,
) (*mcp.CallToolResult, any, error) {
	resp, err := t.Handler.Send(ctx, "get_node", []string{args.NodeID})
	return renderResponse(resp, err)
}

func (t *Tools) handleGetStyles(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ struct{},
) (*mcp.CallToolResult, any, error) {
	resp, err := t.Handler.Send(ctx, "get_styles", nil)
	return renderResponse(resp, err)
}

func (t *Tools) handleGetMetadata(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ struct{},
) (*mcp.CallToolResult, any, error) {
	resp, err := t.Handler.Send(ctx, "get_metadata", nil)
	return renderResponse(resp, err)
}

func (t *Tools) handleGetDesignContext(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	args getDesignContextArgs,
) (*mcp.CallToolResult, any, error) {
	params := make(map[string]interface{})
	if args.Depth > 0 {
		params["depth"] = args.Depth
	}
	resp, err := t.Handler.SendWithParams(ctx, "get_design_context", nil, params)
	return renderResponse(resp, err)
}

func (t *Tools) handleGetVariableDefs(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ struct{},
) (*mcp.CallToolResult, any, error) {
	resp, err := t.Handler.Send(ctx, "get_variable_defs", nil)
	return renderResponse(resp, err)
}

func (t *Tools) handleGetScreenshot(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	args getScreenshotArgs,
) (*mcp.CallToolResult, any, error) {
	params := make(map[string]interface{})
	if args.Format != "" {
		params["format"] = args.Format
	}
	if args.Scale > 0 {
		params["scale"] = args.Scale
	}
	resp, err := t.Handler.SendWithParams(ctx, "get_screenshot", args.NodeIDs, params)
	return renderResponse(resp, err)
}

func renderResponse(resp bridge.Response, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: err.Error()},
			},
			IsError: true,
		}, nil, nil
	}

	payload, marshalErr := json.Marshal(resp.Data)
	if marshalErr != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: marshalErr.Error()},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(payload)},
		},
	}, nil, nil
}
