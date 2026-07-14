package api

import (
	"embed"
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/springsunx/ean13-api/decode"
)

//go:embed web/*
var webFS embed.FS

const (
	// MaxUploadBytes is the maximum upload size in bytes (256 MiB).
	MaxUploadBytes = 256 << 20

	// MaxPixels is the maximum number of pixels allowed (180 million, ~12K×12K).
	MaxPixels = 180_000_000

	// MaxConcurrentDecodes is the maximum number of concurrent decode operations.
	MaxConcurrentDecodes = 2
)

// decodeSem is a semaphore that limits concurrent decode operations.
var decodeSem = make(chan struct{}, MaxConcurrentDecodes)

// DecodeResponse is the JSON response from the decode endpoint.
type DecodeResponse struct {
	Success bool   `json:"success"`
	Text    string `json:"text,omitempty"`
	Format  string `json:"format,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ConfigResponse is the JSON response from the config endpoint.
type ConfigResponse struct {
	MaxUploadBytes     int    `json:"maxUploadBytes"`
	MaxPixels          int    `json:"maxPixels"`
	MaxConcurrent      int    `json:"maxConcurrent"`
	DefaultMaxWidth    int    `json:"defaultMaxWidth"`
	SupportedFormats   []string `json:"supportedFormats"`
}

// NewHandler creates an http.ServeMux with all API routes registered.
func NewHandler() *http.ServeMux {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/decode", handleDecode)
	mux.HandleFunc("/api/config", handleConfig)
	mux.HandleFunc("/health", handleHealth)

	// Serve embedded frontend files
	webSubFS, err := fs.Sub(webFS, "web")
	if err != nil {
		// If embedding fails, log but don't crash - API still works
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend not available", http.StatusInternalServerError)
		})
		return mux
	}

	fileServer := http.FileServer(http.FS(webSubFS))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Serve index.html for root path directly
		if r.URL.Path == "/" {
			data, err := webSubFS.(fs.ReadFileFS).ReadFile("index.html")
			if err != nil {
				http.Error(w, "index.html not found", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(data)
			return
		}

		// Set content type for CSS and JS
		if strings.HasSuffix(r.URL.Path, ".css") {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		} else if strings.HasSuffix(r.URL.Path, ".js") {
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		}

		fileServer.ServeHTTP(w, r)
	})

	return mux
}

// handleHealth responds with a simple health check.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DecodeResponse{Success: true})
}

// handleConfig returns server configuration limits.
func handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ConfigResponse{
		MaxUploadBytes:   MaxUploadBytes,
		MaxPixels:        MaxPixels,
		MaxConcurrent:    MaxConcurrentDecodes,
		DefaultMaxWidth:  decode.DefaultMaxWidth,
		SupportedFormats: []string{"image/png", "image/jpeg"},
	})
}

// handleDecode accepts a multipart form with an "image" file field,
// decodes an EAN-13 barcode from the image, and returns the result as JSON.
// Optional query parameter: maxWidth (default 4096) to control image downscaling.
func handleDecode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(DecodeResponse{
			Success: false,
			Error:   "method not allowed, use POST",
		})
		return
	}

	// Parse optional maxWidth query parameter
	maxWidth := decode.DefaultMaxWidth
	if mwStr := r.URL.Query().Get("maxWidth"); mwStr != "" {
		if mw, err := strconv.Atoi(mwStr); err == nil && mw > 0 && mw <= 16384 {
			maxWidth = mw
		}
	}

	// Parse multipart form with upload size limit
	if err := r.ParseMultipartForm(MaxUploadBytes); err != nil {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		json.NewEncoder(w).Encode(DecodeResponse{
			Success: false,
			Error:   "upload too large (max 256 MiB)",
		})
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(DecodeResponse{
			Success: false,
			Error:   "missing 'image' field in form",
		})
		return
	}
	defer file.Close()

	// Pre-check: decode only the image config (format + dimensions) first.
	// This avoids allocating full pixel data for oversized images.
	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(DecodeResponse{
			Success: false,
			Error:   "failed to read image config: unsupported or corrupted format",
		})
		return
	}

	// Check pixel budget before full decode.
	pixels := cfg.Width * cfg.Height
	if pixels > MaxPixels {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		json.NewEncoder(w).Encode(DecodeResponse{
			Success: false,
			Error:   "image too large (max " + strconv.Itoa(MaxPixels/1_000_000) + " megapixels)",
		})
		return
	}

	// Seek back to the beginning for full decode.
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	// Acquire concurrency slot. Non-blocking check with immediate error if full.
	select {
	case decodeSem <- struct{}{}:
		defer func() { <-decodeSem }()
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(DecodeResponse{
			Success: false,
			Error:   "server busy, too many concurrent requests",
		})
		return
	}

	img, _, err := image.Decode(file)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(DecodeResponse{
			Success: false,
			Error:   "failed to decode image: unsupported or corrupted image format",
		})
		return
	}

	result, err := decode.EAN13WithMaxWidth(img, maxWidth)
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(DecodeResponse{
			Success: false,
			Error:   "no EAN-13 barcode found in image",
		})
		return
	}

	json.NewEncoder(w).Encode(DecodeResponse{
		Success: true,
		Text:    result.Text,
		Format:  result.Format,
	})
}
