package main

import (
	"fmt"

	"github.com/mark3labs/mcp-go/server"
	mcpsrv "github.com/springsunx/ean13-api/mcpserver"
)

func main() {
	s := mcpsrv.NewMCPServer()
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("MCP server error: %v\n", err)
	}
}
