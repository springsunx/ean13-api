package testutil

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"reflect"
	"testing"

	"github.com/springsunx/ean13-api"
)

// NewBinaryBitmapFromFile loads an image file and creates a BinaryBitmap for decoding.
func NewBinaryBitmapFromFile(filename string) *gozxing.BinaryBitmap {
	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		panic(err)
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		panic(err)
	}
	return bmp
}

// TestFile decodes a barcode from an image file and verifies the result.
func TestFile(t testing.TB, reader gozxing.Reader, file, expectText string,
	expectFormat gozxing.BarcodeFormat, hints map[gozxing.DecodeHintType]interface{},
	metadata map[gozxing.ResultMetadataType]interface{}) {
	t.Helper()
	bmp := NewBinaryBitmapFromFile(file)
	result, e := reader.Decode(bmp, hints)
	if e != nil {
		t.Fatalf("TestFile(%v) reader.Decode failed: %v", file, e)
	}
	if txt := result.GetText(); txt != expectText {
		t.Fatalf("TestFile(%v) = \"%v\", wants \"%v\"", file, txt, expectText)
	}
	if format := result.GetBarcodeFormat(); format != expectFormat {
		t.Fatalf("TestFile(%v) format = %v, wants %v", file, format, expectFormat)
	}

	meta := result.GetResultMetadata()
	for k, v := range metadata {
		m, ok := meta[k]
		if !ok {
			t.Fatalf("TestFile(%v) metadata[%v] not found", file, k)
		}
		if !reflect.DeepEqual(m, v) {
			t.Fatalf("TestFile(%v) metadata[%v] = %#v, wants %#v", file, k, m, v)
		}
	}
}
