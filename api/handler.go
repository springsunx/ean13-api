package api

import (
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"

	"github.com/springsunx/ean13-api"
	"github.com/springsunx/ean13-api/oned"
)

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
	mux.HandleFunc("/api/decode", handleDecode)
	mux.HandleFunc("/health", handleHealth)
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
			Error:   "failed to parse form: " + err.Error(),
		})
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(DecodeResponse{
			Success: false,
			Error:   "missing 'image' field in form: " + err.Error(),
		})
		return
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(DecodeResponse{
			Success: false,
			Error:   "failed to decode image: " + err.Error(),
		})
		return
	}

	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(DecodeResponse{
			Success: false,
			Error:   "failed to create bitmap: " + err.Error(),
		})
		return
	}

	reader := oned.NewEAN13Reader()
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}
	result, err := reader.Decode(bmp, hints)
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
		Text:    result.GetText(),
		Format:  result.GetBarcodeFormat().String(),
	})
}
