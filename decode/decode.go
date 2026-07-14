// Package decode provides shared EAN-13 barcode decoding logic.
package decode

import (
	"image"
	"image/color"
	"math"

	gozxing "github.com/springsunx/ean13-api"
	"github.com/springsunx/ean13-api/oned"
)

const (
	// DefaultMaxWidth is the default maximum width for initial decode attempt.
	DefaultMaxWidth = 4096

	// TileSize is the maximum dimension for tiled scanning.
	TileSize = 4096

	// TileOverlap is the overlap ratio between adjacent tiles (0.2 = 20%).
	TileOverlap = 0.2

	// MaxParallelTiles is the maximum number of tiles decoded in parallel.
	MaxParallelTiles = 2
)

// Result holds the decoded barcode information.
type Result struct {
	Text   string
	Format string
}

// EAN13 decodes an EAN-13 barcode from the given image.
// Returns the decoded result or an error if no barcode is found.
func EAN13(img image.Image) (*Result, error) {
	return EAN13WithMaxWidth(img, DefaultMaxWidth)
}

// EAN13WithMaxWidth decodes an EAN-13 barcode using a tiered strategy:
//
//  1. If the long edge ≤ maxWidth, decode directly (fast path).
//  2. Otherwise, generate a grayscale preview at maxWidth and try decode.
//  3. If that fails, tile the original image into overlapping blocks and
//     scan each tile with a quick pass, then a full pass with TRY_HARDER.
//     Up to MaxParallelTiles are processed concurrently; the first success
//     cancels all remaining work.
func EAN13WithMaxWidth(img image.Image, maxWidth int) (*Result, error) {
	bounds := img.Bounds()
	longEdge := bounds.Dx()
	if bounds.Dy() > longEdge {
		longEdge = bounds.Dy()
	}

	if longEdge <= maxWidth {
		// Image is small enough — direct decode with TRY_HARDER (includes 90° rotation).
		return tryDecode(img, true)
	}

	// Large image: generate grayscale preview at maxWidth.
	preview := downscaleToGray(img, maxWidth)

	// Try horizontal + vertical on the preview (TRY_HARDER handles rotation).
	if result, err := tryDecode(preview, true); err == nil {
		return result, nil
	}

	// Preview failed — fall back to tiled scanning of the original image.
	return tryDecodeTiled(img)
}

// tryDecode attempts to decode an EAN-13 barcode from the given image.
// If tryHarder is true, scans all rows and attempts 90° rotation.
func tryDecode(img image.Image, tryHarder bool) (*Result, error) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return nil, err
	}

	reader := oned.NewEAN13Reader()
	hints := map[gozxing.DecodeHintType]interface{}{}
	if tryHarder {
		hints[gozxing.DecodeHintType_TRY_HARDER] = true
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

// tryDecodeTiled splits a large image into overlapping tiles and scans each one.
// Tiles are processed in parallel (up to MaxParallelTiles). The first successful
// decode cancels all remaining work.
func tryDecodeTiled(img image.Image) (*Result, error) {
	bounds := img.Bounds()
	tileRects := generateTiles(bounds, TileSize, TileOverlap)

	type tileResult struct {
		result *Result
		err    error
	}

	// Buffered channel for results. One extra slot for the sentinel nil on cancel.
	resultCh := make(chan tileResult, len(tileRects)+1)
	cancel := make(chan struct{})
	done := make(chan struct{})

	// Worker pool with bounded parallelism.
	sem := make(chan struct{}, MaxParallelTiles)

	go func() {
		defer close(done)
		for _, r := range tileRects {
			select {
			case <-cancel:
				return
			case sem <- struct{}{}:
			}

			tileRect := r
			go func() {
				defer func() { <-sem }()

				tile := subImage(img, tileRect)

				// Quick scan: no TRY_HARDER (15 rows only).
				if result, err := tryDecode(tile, false); err == nil {
					select {
					case resultCh <- tileResult{result, nil}:
					case <-cancel:
					}
					return
				}

				// Full scan: TRY_HARDER (all rows + 90° rotation).
				if result, err := tryDecode(tile, true); err == nil {
					select {
					case resultCh <- tileResult{result, nil}:
					case <-cancel:
					}
					return
				}
			}()
		}
	}()

	// Wait for first success or all failures.
	go func() {
		<-done
		close(resultCh)
	}()

	// Collect results: first success wins.
	for tr := range resultCh {
		if tr.result != nil {
			close(cancel)
			return tr.result, nil
		}
	}

	return nil, gozxing.NewNotFoundException()
}

// generateTiles splits a rectangle into overlapping tiles of at most tileSize×tileSize.
func generateTiles(bounds image.Rectangle, tileSize int, overlap float64) []image.Rectangle {
	w := bounds.Dx()
	h := bounds.Dy()

	if w <= tileSize && h <= tileSize {
		return []image.Rectangle{bounds}
	}

	step := int(float64(tileSize) * (1.0 - overlap))
	if step < 1 {
		step = 1
	}

	var rects []image.Rectangle
	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		for x := bounds.Min.X; x < bounds.Max.X; x += step {
			x1 := x
			y1 := y
			x2 := x + tileSize
			y2 := y + tileSize
			if x2 > bounds.Max.X {
				x2 = bounds.Max.X
			}
			if y2 > bounds.Max.Y {
				y2 = bounds.Max.Y
			}
			rects = append(rects, image.Rect(x1, y1, x2, y2))
		}
	}
	return rects
}

// subImage returns a sub-view of the image without copying pixel data.
// For image types that support SubImage (RGBA, NRGBA, Gray, YCbCr), this is zero-copy.
func subImage(img image.Image, rect image.Rectangle) image.Image {
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	if si, ok := img.(subImager); ok {
		return si.SubImage(rect)
	}
	// Fallback: copy the region into a new RGBA image.
	bounds := img.Bounds()
	newImg := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
				newImg.Set(x-rect.Min.X, y-rect.Min.Y, img.At(x, y))
			}
		}
	}
	return newImg
}

// downscaleToGray resizes an image to fit within maxWidth, outputting *image.Gray directly.
// This avoids the intermediate RGBA allocation, saving 4× memory during downscaling.
func downscaleToGray(img image.Image, maxDim int) *image.Gray {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	longEdge := width
	if height > longEdge {
		longEdge = height
	}

	if longEdge <= maxDim {
		return toGray(img)
	}

	// Scale so that the long edge equals maxDim.
	var newWidth, newHeight int
	if width >= height {
		newWidth = maxDim
		newHeight = int(math.Round(float64(maxDim) * float64(height) / float64(width)))
	} else {
		newHeight = maxDim
		newWidth = int(math.Round(float64(maxDim) * float64(width) / float64(height)))
	}
	if newWidth < 1 {
		newWidth = 1
	}
	if newHeight < 1 {
		newHeight = 1
	}

	gray := image.NewGray(image.Rect(0, 0, newWidth, newHeight))
	xRatio := float64(width) / float64(newWidth)
	yRatio := float64(height) / float64(newHeight)

	// Type-switch for fast luminance sampling during downscaling.
	switch src := img.(type) {
	case *image.YCbCr:
		// JPEG: sample Y channel directly.
		for y := 0; y < newHeight; y++ {
			srcY := int(float64(y) * yRatio)
			if srcY >= height-1 {
				srcY = height - 2
			}
			for x := 0; x < newWidth; x++ {
				srcX := int(float64(x) * xRatio)
				if srcX >= width-1 {
					srcX = width - 2
				}
				// Use nearest-neighbor on Y channel (sufficient for barcode detection)
				off := (srcY+bounds.Min.Y-src.Rect.Min.Y)*src.YStride + (srcX + bounds.Min.X - src.Rect.Min.X)
				gray.SetGray(x, y, color.Gray{Y: src.Y[off]})
			}
		}
	case *image.Gray:
		for y := 0; y < newHeight; y++ {
			srcY := int(float64(y) * yRatio)
			if srcY >= height-1 {
				srcY = height - 2
			}
			for x := 0; x < newWidth; x++ {
				srcX := int(float64(x) * xRatio)
				if srcX >= width-1 {
					srcX = width - 2
				}
				off := (srcY+bounds.Min.Y-src.Rect.Min.Y)*src.Stride + (srcX + bounds.Min.X - src.Rect.Min.X)
				gray.SetGray(x, y, color.Gray{Y: src.Pix[off]})
			}
		}
	default:
		// Generic: sample via At() and convert to luminance.
		for y := 0; y < newHeight; y++ {
			srcY := int(float64(y) * yRatio)
			if srcY >= height-1 {
				srcY = height - 2
			}
			for x := 0; x < newWidth; x++ {
				srcX := int(float64(x) * xRatio)
				if srcX >= width-1 {
					srcX = width - 2
				}
				r, g, b, _ := img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY).RGBA()
				lum := (r + 2*g + b) * 255 / (4 * 0xffff)
				gray.SetGray(x, y, color.Gray{Y: uint8(lum)})
			}
		}
	}

	return gray
}

// toGray converts an image to *image.Gray without resizing.
func toGray(img image.Image) *image.Gray {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	if g, ok := img.(*image.Gray); ok {
		if g.Rect == bounds {
			return g
		}
		// Different bounds — need to copy.
		out := image.NewGray(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			copy(out.Pix[y*out.Stride:y*out.Stride+w],
				g.Pix[(y+bounds.Min.Y-g.Rect.Min.Y)*g.Stride+(bounds.Min.X-g.Rect.Min.X):
					(y+bounds.Min.Y-g.Rect.Min.Y)*g.Stride+(bounds.Min.X-g.Rect.Min.X)+w])
		}
		return out
	}

	out := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			lum := (r + 2*g + b) * 255 / (4 * 0xffff)
			out.SetGray(x, y, color.Gray{Y: uint8(lum)})
		}
	}
	return out
}
