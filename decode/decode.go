// Package decode provides shared EAN-13 barcode decoding logic.
package decode

import (
	"image"

	gozxing "github.com/springsunx/ean13-api"
	"github.com/springsunx/ean13-api/oned"
)

// Result holds the decoded barcode information.
type Result struct {
	Text   string
	Format string
}

// EAN13 decodes an EAN-13 barcode from the given image.
// Returns the decoded result or an error if no barcode is found.
func EAN13(img image.Image) (*Result, error) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return nil, err
	}

	reader := oned.NewEAN13Reader()
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}
	result, err := reader.Decode(bmp, hints)
	if err != nil {
		return nil, err
	}

	return &Result{
		Text:   result.GetText(),
		Format: result.GetBarcodeFormat().String(),
	}, nil
}
