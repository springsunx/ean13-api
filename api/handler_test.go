package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// createTestBarcodeImage creates a simple test image.
// Note: this creates a plain white image which will NOT contain a valid barcode.
// It is used to test the API error handling path.
func createTestBarcodeImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 300, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 300; x++ {
			img.Set(x, y, color.White)
		}
	}
	return img
}

func TestHandleHealth(t *testing.T) {
	handler := NewHandler()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("health check returned status %d, want %d", w.Code, http.StatusOK)
	}

	var resp DecodeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("health check response success=false")
	}
}

func TestHandleDecode_MethodNotAllowed(t *testing.T) {
	handler := NewHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/decode", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("returned status %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleDecode_MissingImage(t *testing.T) {
	handler := NewHandler()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	// Add a non-image field
	writer.WriteField("foo", "bar")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/decode", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("returned status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleDecode_NoBarcode(t *testing.T) {
	handler := NewHandler()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("image", "test.png")
	img := createTestBarcodeImage()
	png.Encode(part, img)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/decode", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("returned status %d, want %d", w.Code, http.StatusUnprocessableEntity)
	}

	var resp DecodeResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Success {
		t.Fatalf("expected success=false for image without barcode")
	}
}

func TestHandleDecode_WithRealBarcode(t *testing.T) {
	handler := NewHandler()

	// Use the EAN-13 test image from the testdata directory
	_, thisFile, _, _ := runtime.Caller(0)
	testImagePath := filepath.Join(filepath.Dir(thisFile), "..", "oned", "testdata", "ean13", "1.png")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("image", "test.png")

	imgFile, err := os.Open(testImagePath)
	if err != nil {
		t.Skipf("could not open test image: %v", err)
	}
	defer imgFile.Close()

	img, _, err := image.Decode(imgFile)
	if err != nil {
		t.Skipf("could not decode test image: %v", err)
	}
	png.Encode(part, img)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/decode", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("returned status %d, want %d", w.Code, http.StatusOK)
	}

	var resp DecodeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got error: %s", resp.Error)
	}
	if resp.Text != "8413000065504" {
		t.Fatalf("decoded text = %q, want %q", resp.Text, "8413000065504")
	}
	if resp.Format != "EAN_13" {
		t.Fatalf("format = %q, want %q", resp.Format, "EAN_13")
	}
}
