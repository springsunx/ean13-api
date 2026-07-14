package gozxing

import (
	"image"

	"errors"
)

func NewBinaryBitmapFromImage(img image.Image) (*BinaryBitmap, error) {
	src := NewLuminanceSourceFromImage(img)
	return NewBinaryBitmap(NewHybridBinarizer(src))
}

type GoImageLuminanceSource struct {
	*RGBLuminanceSource
}

func NewLuminanceSourceFromImage(img image.Image) LuminanceSource {
	rect := img.Bounds()
	width := rect.Max.X - rect.Min.X
	height := rect.Max.Y - rect.Min.Y

	luminance := make([]byte, width*height)
	index := 0
	// Optimize special cases.
	switch img := img.(type) {
	case *image.YCbCr:
		// Fast path: YCbCr images (produced by image/jpeg).
		// The Y channel IS luminance (ITU-R BT.601), so we can copy it directly.
		// This is the fastest possible path for JPEG images.
		stride := img.YStride
		baseOffset := (rect.Min.Y-img.Rect.Min.Y)*stride + (rect.Min.X - img.Rect.Min.X)
		for y := 0; y < height; y++ {
			srcOffset := baseOffset + y*stride
			copy(luminance[index:index+width], img.Y[srcOffset:srcOffset+width])
			index += width
		}
	case *image.Gray:
		// Fast path: direct Pix slice access for Gray images (common from JPEG grayscale)
		stride := img.Stride
		baseOffset := (rect.Min.Y-img.Rect.Min.Y)*stride + (rect.Min.X - img.Rect.Min.X)
		for y := 0; y < height; y++ {
			srcOffset := baseOffset + y*stride
			copy(luminance[index:index+width], img.Pix[srcOffset:srcOffset+width])
			index += width
		}
	case *image.NRGBA:
		// Fast path: direct Pix slice access for NRGBA images (common from PNG)
		// Uses the same luminance formula as the default path: (R + 2*G + B) / 4
		stride := img.Stride
		baseOffset := (rect.Min.Y-img.Rect.Min.Y)*stride + (rect.Min.X-img.Rect.Min.X)*4
		for y := 0; y < height; y++ {
			srcOffset := baseOffset + y*stride
			for x := 0; x < width; x++ {
				pi := srcOffset + x*4
				r := int(img.Pix[pi])
				g := int(img.Pix[pi+1])
				b := int(img.Pix[pi+2])
				a := int(img.Pix[pi+3])
				lum := (r + 2*g + b) / 4
				// Alpha blend with white background
				luminance[index] = byte((lum*a + (255-a)*255) / 255)
				index++
			}
		}
	case *image.RGBA:
		// Fast path: direct Pix slice access for RGBA images (premultiplied alpha)
		// For fully opaque pixels (A=255), luminance = (R + 2*G + B) / 4
		stride := img.Stride
		baseOffset := (rect.Min.Y-img.Rect.Min.Y)*stride + (rect.Min.X-img.Rect.Min.X)*4
		for y := 0; y < height; y++ {
			srcOffset := baseOffset + y*stride
			for x := 0; x < width; x++ {
				pi := srcOffset + x*4
				r := int(img.Pix[pi])
				g := int(img.Pix[pi+1])
				b := int(img.Pix[pi+2])
				a := int(img.Pix[pi+3])
				if a == 0 {
					luminance[index] = 255
				} else {
					// Un-premultiply, compute luminance, then alpha-blend with white
					ur := r * 255 / a
					ug := g * 255 / a
					ub := b * 255 / a
					lum := (ur + 2*ug + ub) / 4
					luminance[index] = byte((lum*a + (255-a)*255) / 255)
				}
				index++
			}
		}
	case image.RGBA64Image:
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			for x := rect.Min.X; x < rect.Max.X; x++ {
				r, g, b, a := img.RGBA64At(x, y).RGBA()
				lum := (r + 2*g + b) * 255 / (4 * 0xffff)
				luminance[index] = byte((lum*a + (0xffff-a)*255) / 0xffff)
				index++
			}
		}
	default:
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			for x := rect.Min.X; x < rect.Max.X; x++ {
				r, g, b, a := img.At(x, y).RGBA()
				lum := (r + 2*g + b) * 255 / (4 * 0xffff)
				luminance[index] = byte((lum*a + (0xffff-a)*255) / 0xffff)
				index++
			}
		}
	}

	return &GoImageLuminanceSource{&RGBLuminanceSource{
		LuminanceSourceBase{width, height},
		luminance,
		width,
		height,
		0,
		0,
	}}
}

func (this *GoImageLuminanceSource) Crop(left, top, width, height int) (LuminanceSource, error) {
	cropped, e := this.RGBLuminanceSource.Crop(left, top, width, height)
	if e != nil {
		return nil, e
	}
	return &GoImageLuminanceSource{cropped.(*RGBLuminanceSource)}, nil
}

func (this *GoImageLuminanceSource) Invert() LuminanceSource {
	return LuminanceSourceInvert(this)
}

func (this *GoImageLuminanceSource) IsRotateSupported() bool {
	return true
}

func (this *GoImageLuminanceSource) RotateCounterClockwise() (LuminanceSource, error) {
	width := this.GetWidth()
	height := this.GetHeight()
	top := this.top
	left := this.left
	dataWidth := this.dataWidth
	oldLuminas := this.RGBLuminanceSource.luminances
	newLuminas := make([]byte, width*height)

	for j := 0; j < width; j++ {
		x := left + width - 1 - j
		for i := 0; i < height; i++ {
			y := top + i
			newLuminas[j*height+i] = oldLuminas[y*dataWidth+x]
		}
	}
	return &GoImageLuminanceSource{&RGBLuminanceSource{
		LuminanceSourceBase{height, width},
		newLuminas,
		height,
		width,
		0,
		0,
	}}, nil
}

func (this *GoImageLuminanceSource) RotateCounterClockwise45() (LuminanceSource, error) {
	return nil, errors.New("RotateCounterClockwise45 is not implemented")
}
