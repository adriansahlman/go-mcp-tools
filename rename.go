package go_mcp_tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func AddRenameTool(mcpServer *server.MCPServer) {
	// handleRenameSymbolTool handles the rename symbol tool requests
	handleRename := func(
		ctx context.Context,
		request mcp.CallToolRequest,
	) (*mcp.CallToolResult, error) {
		arguments := request.GetArguments()

		filePath, ok := arguments["file_path"].(string)
		if !ok || filePath == "" {
			return nil, fmt.Errorf("file_path argument is required and must be a string")
		}

		lineNumberFloat, ok := arguments["line_number"].(float64)
		if !ok {
			return nil, fmt.Errorf(
				"line_number argument is required and must be a number",
			)
		}
		lineNumber := int(lineNumberFloat)

		oldName, ok := arguments["old_name"].(string)
		if !ok || oldName == "" {
			return nil, fmt.Errorf("old_name argument is required and must be a string")
		}

		newName, ok := arguments["new_name"].(string)
		if !ok || newName == "" {
			return nil, fmt.Errorf("new_name argument is required and must be a string")
		}

		// Call the rename function
		result, err := Rename(filePath, lineNumber, oldName, newName)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error renaming symbol: %v", err),
					},
				},
				IsError: true,
			}, nil
		}

		// Format the result
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: result,
				},
			},
		}, nil
	}

	mcpServer.AddTool(mcp.NewTool("rename",
		mcp.WithDescription("Renames a Go symbol throughout a file"),
		mcp.WithString("file_path",
			mcp.Description("Path to the Go file containing the symbol to rename"),
			mcp.Required(),
		),
		mcp.WithNumber("line_number",
			mcp.Description("Line number where the symbol is defined"),
			mcp.Required(),
		),
		mcp.WithString("old_name",
			mcp.Description("Current name of the symbol to rename"),
			mcp.Required(),
		),
		mcp.WithString("new_name",
			mcp.Description("New name for the symbol"),
			mcp.Required(),
		),
	), handleRename)
}

func Rename(
	filePath string,
	lineNumber int,
	symbolName string,
	newName string,
) (string, error) {
	if filePath == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}

	if lineNumber <= 0 {
		return "", fmt.Errorf("line number must be positive, got %d", lineNumber)
	}

	if symbolName == "" {
		return "", fmt.Errorf("symbol name cannot be empty")
	}

	if newName == "" {
		return "", fmt.Errorf("new name cannot be empty")
	}

	if symbolName == newName {
		return fmt.Sprintf("Symbol '%s' already has the desired name", symbolName), nil
	}

	// Find the column position of the symbol at the given line
	position, err := createGoplsPosition(filePath, lineNumber, symbolName)
	if err != nil {
		return "", err
	}

	output, err := executeGoplsCommand("rename", "-w", position, newName)
	if err != nil {
		return "", fmt.Errorf(
			"failed to rename symbol '%s' at %s: %w",
			symbolName,
			position,
			err,
		)
	}
	if output == "" {
		output = fmt.Sprintf("Symbol '%s' renamed to '%s'", symbolName, newName)
	}
	return output, nil
}
