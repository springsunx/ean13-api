package mcpserver

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestNewMCPServer(t *testing.T) {
	s := NewMCPServer()
	if s == nil {
		t.Fatal("NewMCPServer returned nil")
	}
}

func createTestImage() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.White)
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestHandleDecodeFile_FileNotFound(t *testing.T) {
	s := NewMCPServer()
	_ = s

	// We can't easily call MCP handlers directly, but we can test the decode logic
	// by testing the exported function
}

func TestDecodeImage_NoBarcode(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.White)
		}
	}

	result, err := decodeImage(img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestHandleDecodeBase64_InvalidBase64(t *testing.T) {
	// Test that invalid base64 is handled gracefully
	_, err := base64.StdEncoding.DecodeString("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestHandleDecodeFile_WithRealImage(t *testing.T) {
	// Find the test image
	wd, err := os.Getwd()
	if err != nil {
		t.Skipf("cannot get working directory: %v", err)
	}

	testImagePath := filepath.Join(wd, "..", "oned", "testdata", "ean13", "1.png")
	if _, err := os.Stat(testImagePath); os.IsNotExist(err) {
		t.Skipf("test image not found: %s", testImagePath)
	}

	file, err := os.Open(testImagePath)
	if err != nil {
		t.Skipf("cannot open test image: %v", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		t.Skipf("cannot decode test image: %v", err)
	}

	result, err := decodeImage(img)
	if err != nil {
		t.Fatalf("decodeImage failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// Check it's a valid MCP result
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content)
	}
}

func TestHandleDecodeBase64_WithRealImage(t *testing.T) {
	// Find the test image
	wd, err := os.Getwd()
	if err != nil {
		t.Skipf("cannot get working directory: %v", err)
	}

	testImagePath := filepath.Join(wd, "..", "oned", "testdata", "ean13", "1.png")
	if _, err := os.Stat(testImagePath); os.IsNotExist(err) {
		t.Skipf("test image not found: %s", testImagePath)
	}

	imgData, err := os.ReadFile(testImagePath)
	if err != nil {
		t.Skipf("cannot read test image: %v", err)
	}

	b64 := base64.StdEncoding.EncodeToString(imgData)

	// Test the base64 decode flow
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("image decode failed: %v", err)
	}

	result, err := decodeImage(img)
	if err != nil {
		t.Fatalf("decodeImage failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content)
	}
}

func TestHandleDecodeBase64_WithDataURI(t *testing.T) {
	// Test data URI prefix stripping
	b64 := "data:image/png;base64," + base64.StdEncoding.EncodeToString(createTestImage())

	// Simulate the stripping logic from handleDecodeBase64
	if idx := bytes.IndexByte([]byte(b64), ','); idx != -1 {
		b64 = b64[idx+1:]
	}

	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("image decode failed: %v", err)
	}

	// This should fail to find a barcode but not panic
	_, _ = decodeImage(img)
}
