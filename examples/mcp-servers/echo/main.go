package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create a new MCP server
	s := server.NewMCPServer(
		"echo-server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Add echo tool
	echoTool := mcp.NewTool("echo",
		mcp.WithDescription("Echoes back the input text"),
		mcp.WithString("message",
			mcp.Required(),
			mcp.Description("The message to echo back"),
		),
	)

	s.AddTool(echoTool, handleEcho)

	// Add uppercase tool
	uppercaseTool := mcp.NewTool("uppercase",
		mcp.WithDescription("Converts text to uppercase"),
		mcp.WithString("text",
			mcp.Required(),
			mcp.Description("The text to convert to uppercase"),
		),
	)

	s.AddTool(uppercaseTool, handleUppercase)

	// Add add_numbers tool
	addTool := mcp.NewTool("add_numbers",
		mcp.WithDescription("Adds two numbers together"),
		mcp.WithNumber("a",
			mcp.Required(),
			mcp.Description("First number"),
		),
		mcp.WithNumber("b",
			mcp.Required(),
			mcp.Description("Second number"),
		),
	)

	s.AddTool(addTool, handleAdd)

	// Start the server
	if err := server.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}

func handleEcho(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	message, err := request.RequireString("message")
	if err != nil {
		return mcp.NewToolResultError("message must be a string"), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Echo: %s", message)), nil
}

func handleUppercase(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	text, err := request.RequireString("text")
	if err != nil {
		return mcp.NewToolResultError("text must be a string"), nil
	}

	// Simple uppercase conversion
	uppercased := ""
	for _, r := range text {
		if r >= 'a' && r <= 'z' {
			uppercased += string(r - 32)
		} else {
			uppercased += string(r)
		}
	}

	return mcp.NewToolResultText(uppercased), nil
}

func handleAdd(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a := request.GetFloat("a", 0)
	b := request.GetFloat("b", 0)

	result := a + b
	return mcp.NewToolResultText(fmt.Sprintf("Result: %.2f", result)), nil
}
