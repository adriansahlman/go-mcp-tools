package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	go_mcp_tools "github.com/adriansahlman/go-mcp-tools"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "server":
		runServer(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Go MCP Tools - Model Context Protocol tools for Go development")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run cmd/main.go server [flags]     Start MCP server")
	fmt.Println()
	fmt.Println("Server Commands:")
	fmt.Println("  server --transport stdio             Start stdio server (default)")
	fmt.Println("  server --transport http              Start HTTP server")
	fmt.Println("         --host localhost              HTTP host (default: localhost)")
	fmt.Println("         --port 8080                   HTTP port (default: 8080)")
	fmt.Println("         --disable-tool <tool>         Disable specific tool")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Start stdio server")
	fmt.Println("  go run cmd/main.go server")
	fmt.Println()
	fmt.Println("  # Start HTTP server")
	fmt.Println("  go run cmd/main.go server --transport http --port 9000")
}

func runServer(args []string) {
	fs := flag.NewFlagSet("server", flag.ExitOnError)

	transport := fs.String("transport", "stdio", "Transport type (stdio or http)")
	host := fs.String("host", "localhost", "Host for HTTP transport")
	port := fs.String("port", "8080", "Port for HTTP transport")

	if err := fs.Parse(args); err != nil {
		log.Fatalf("Error parsing server flags: %v", err)
	}

	mcpServer := go_mcp_tools.NewMCPServer(nil)

	// Start serving
	if *transport == "http" {
		fmt.Printf("Starting HTTP server on %s:%s/mcp\n", *host, *port)
		if err := go_mcp_tools.ServeHTTP(mcpServer, *host, *port); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	} else {
		// no printing to stdio as it is used for machine communication
		if err := go_mcp_tools.ServeStdio(mcpServer); err != nil {
			log.Fatalf("Stdio server error: %v", err)
		}
	}
}
