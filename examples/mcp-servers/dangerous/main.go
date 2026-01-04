package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create a new MCP server for testing dangerous operations
	s := server.NewMCPServer(
		"dangerous-server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Add safe echo tool (should be allowed)
	echoTool := mcp.NewTool("echo",
		mcp.WithDescription("Safely echoes back the input text"),
		mcp.WithString("message",
			mcp.Required(),
			mcp.Description("The message to echo back"),
		),
	)
	s.AddTool(echoTool, handleEcho)

	// Add file creation tool (should require approval)
	createFileTool := mcp.NewTool("create_file",
		mcp.WithDescription("Creates a new file with specified content"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("File path to create"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("File content"),
		),
	)
	s.AddTool(createFileTool, handleCreateFile)

	// Add dangerous delete tool (should be blocked)
	deleteFileTool := mcp.NewTool("delete_file",
		mcp.WithDescription("Deletes a file (DANGEROUS!)"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("File path to delete"),
		),
	)
	s.AddTool(deleteFileTool, handleDeleteFile)

	// Add shell execution tool (should be blocked)
	shellTool := mcp.NewTool("exec_shell",
		mcp.WithDescription("Executes shell commands (VERY DANGEROUS!)"),
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description("Shell command to execute"),
		),
	)
	s.AddTool(shellTool, handleShell)

	// Add network tool (should require approval)
	httpTool := mcp.NewTool("http_request",
		mcp.WithDescription("Makes HTTP requests"),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("URL to request"),
		),
	)
	s.AddTool(httpTool, handleHTTP)

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

func handleCreateFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := request.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path must be a string"), nil
	}

	content, err := request.RequireString("content")
	if err != nil {
		return mcp.NewToolResultError("content must be a string"), nil
	}

	// This is a dangerous operation - writing files
	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create file: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("File created: %s", path)), nil
}

func handleDeleteFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := request.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path must be a string"), nil
	}

	// This is a very dangerous operation - deleting files
	err = os.Remove(path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete file: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("File deleted: %s", path)), nil
}

func handleShell(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	command, err := request.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError("command must be a string"), nil
	}

	// This is extremely dangerous - executing arbitrary shell commands
	return mcp.NewToolResultError(fmt.Sprintf("BLOCKED: Shell execution is disabled for security. Command was: %s", command)), nil
}

func handleHTTP(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	url, err := request.RequireString("url")
	if err != nil {
		return mcp.NewToolResultError("url must be a string"), nil
	}

	// This could be dangerous - making network requests
	return mcp.NewToolResultText(fmt.Sprintf("Would make HTTP request to: %s (simulated for security)", url)), nil
}
