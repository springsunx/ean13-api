package api

import (
	"embed"
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"net/http"
	"strings"

	"github.com/springsunx/ean13-api/decode"
)

//go:embed web/*
var webFS embed.FS

// DecodeResponse is the JSON response from the decode endpoint.
type DecodeResponse struct {
	Success bool   `json:"success"`
	Text    string `json:"text,omitempty"`
	Format  string `json:"format,omitempty"`
	Error   string `json:"error,omitempty"`
}

// NewHandler creates an http.ServeMux with all API routes registered.
func NewHandler() *http.ServeMux {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/decode", handleDecode)
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

// handleDecode accepts a multipart form with an "image" file field,
// decodes an EAN-13 barcode from the image, and returns the result as JSON.
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

	// Parse multipart form (max 10 MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(DecodeResponse{
			Success: false,
			Error:   "failed to parse form data",
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

	img, _, err := image.Decode(file)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(DecodeResponse{
			Success: false,
			Error:   "failed to decode image: unsupported or corrupted image format",
		})
		return
	}

	result, err := decode.EAN13(img)
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
