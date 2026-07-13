package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/springsunx/ean13-api/api"
	mcpsrv "github.com/springsunx/ean13-api/mcpserver"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// MCP StreamableHTTP handler
	mcp := mcpsrv.NewMCPServer()
	httpMCP := mcpserver.NewStreamableHTTPServer(mcp)

	mux := http.NewServeMux()

	// All routes handled by api handler (includes frontend, API, and health)
	apiMux := api.NewHandler()
	mux.Handle("/", apiMux)

	// MCP HTTP endpoint
	mux.Handle("/mcp", httpMCP)

	addr := ":" + port
	fmt.Printf("EAN-13 barcode API server starting on %s\n", addr)
	fmt.Printf("  Frontend: http://localhost%s/\n", addr)
	fmt.Printf("  REST API: POST /api/decode, GET /health\n")
	fmt.Printf("  MCP:      POST /mcp  (StreamableHTTP)\n")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
