package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// Configure server with timeouts.
	// ReadTimeout: 2 minutes — large images may take time to upload.
	// ReadHeaderTimeout: 5 seconds — headers should arrive quickly.
	// IdleTimeout: 60 seconds — close idle keep-alive connections.
	// WriteTimeout: 0 (not set) — MCP long connections must not be killed by a global write timeout.
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       2 * time.Minute,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}

	fmt.Printf("EAN-13 barcode API server starting on %s\n", addr)
	fmt.Printf("  Frontend: http://localhost%s/\n", addr)
	fmt.Printf("  REST API: POST /api/decode, GET /health, GET /api/config\n")
	fmt.Printf("  MCP:      POST /mcp  (StreamableHTTP)\n")

	// Graceful shutdown: listen for SIGTERM/SIGINT.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		sig := <-sigCh
		log.Printf("received signal %v, shutting down gracefully...", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
	log.Println("server stopped")
}
