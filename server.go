package go_mcp_tools

import (
	"github.com/mark3labs/mcp-go/server"
)

// ServerConfig holds configuration for the MCP server
type ServerConfig struct {
	Name    string
	Version string
}

// Transport defines the server transport method
type Transport string

const (
	StdioTransport Transport = "stdio"
	HTTPTransport  Transport = "http"
)

// DefaultServerConfig returns a default server configuration
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Name:    "go-mcp-tools",
		Version: "1.0.0",
	}
}

// NewMCPServer creates a new MCP server with the specified configuration
func NewMCPServer(config *ServerConfig) *server.MCPServer {
	if config == nil {
		config = DefaultServerConfig()
	}

	mcpServer := server.NewMCPServer(
		config.Name,
		config.Version,
		server.WithToolCapabilities(true),
	)
	AddInspectTool(mcpServer)
	AddRenameTool(mcpServer)
	return mcpServer
}

// ServeStdio starts the MCP server on stdio transport
func ServeStdio(mcpServer *server.MCPServer) error {
	return server.ServeStdio(mcpServer)
}

// ServeHTTP starts the MCP server on HTTP transport at the specified address
func ServeHTTP(mcpServer *server.MCPServer, host string, port string) error {
	addr := host + ":" + port
	httpServer := server.NewStreamableHTTPServer(mcpServer)
	return httpServer.Start(addr)
}
