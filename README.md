# Go MCP Tools

Collection of tools that I feel are missing when using Copilot agent mode in Go projects.

No tools require the column index for symbols like the gopls cli, as the coding agent does not have easy access to this information and is terrible at counting. Instead it can pass the symbol name and we look up the column index instead where needed.

The tools may be served via stdio or http. The [mark3labs MCP server implementation](https://github.com/mark3labs/mcp-go) is used for the server.

## Tools

### Inspect
Look at a package, file, or symbol and get a summary. The summary leverages gopls and go/ast for adding useful information such as references, implementers, scopes, call hierarchies e.t.c.

### Rename
Rename a symbol. Basically just calls `gopls rename`.

## Usage
May be compiled or run directly using `go`, its entrypoint being [cmd/main.go](cmd/main.go).

### VSCode
In your settings json file, add the following:
```json
    "mcp": {
        "servers": {
            "go": {
                "type": "stdio",
                "command": "/bin/bash",
                "args": [
                    "-c",
                    "go run /path/to/go-mcp-tools/cmd/main.go server",
                ]
            },
        }
    },
```
You can use Ctrl+Shift+P or Cmd+Shift+P to select `MCP: List Servers`, select the go server and start it.

Now the tools should show up when you configure available tools for your coding agent.

### Other Editors/Coding Applications
Look up how to integrate MCP tools with the application you are using and use either stdio or http transport.

## Disclaimers
80% of this project is written by Claude, with heavy supervision.
