package gozxing

import (
	"image"
	"image/color"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	errors "golang.org/x/xerrors"
)

type testBinarizer struct {
	source LuminanceSource
}

func (this *testBinarizer) GetLuminanceSource() LuminanceSource {
	return this.source
}
func (this *testBinarizer) GetBlackRow(y int, row *BitArray) (*BitArray, error) {
	width := this.GetWidth()
	if row.GetSize() < width {
		row = NewBitArray(width)
	} else {
		row.Clear()
	}
	rawrow, e := this.source.GetRow(y, make([]byte, width))
	if e != nil {
		return row, e
	}
	for i, v := range rawrow {
		if v < 128 {
			row.Set(i)
		}
	}
	return row, nil
}
func (this *testBinarizer) GetBlackMatrix() (*BitMatrix, error) {
	width := this.GetWidth()
	height := this.GetHeight()
	matrix, _ := NewBitMatrix(width, height)
	row := NewBitArray(width)
	for y := 0; y < height; y++ {
		row, _ = this.GetBlackRow(y, row)
		for x := 0; x < width; x++ {
			if row.Get(x) {
				matrix.Set(x, y)
			}
		}
	}
	return matrix, nil
}
func (this *testBinarizer) CreateBinarizer(source LuminanceSource) Binarizer {
	return &testBinarizer{source}
}
func (this *testBinarizer) GetWidth() int {
	return this.source.GetWidth()
}
func (this *testBinarizer) GetHeight() int {
	return this.source.GetHeight()
}

func TestBinaryBitmap(t *testing.T) {
	if _, e := NewBinaryBitmap(nil); e == nil {
		t.Fatalf("NewBinaryBitmap must be error")
	}

	binarizer := &testBinarizer{newTestLuminanceSource(16)}
	bmp, e := NewBinaryBitmap(binarizer)
	if e != nil {
		t.Fatalf("NewBinaryBitmap return error, %v", e)
	}
	if w, h := bmp.GetWidth(), bmp.GetHeight(); w != 16 || h != 16 {
		t.Fatalf("width,height = %v,%v, expect 16,16", w, h)
	}

	arr, e := bmp.GetBlackRow(0, NewBitArray(16))
	if e != nil {
		t.Fatalf("GetBlackRow returns error, %v", e)
	}
	for i := 0; i < arr.GetSize(); i++ {
		if arr.Get(i) != (i < 8) {
			t.Fatalf("BlackRow.Get(%v) = %v, expect %v", i, arr.Get(i), (i < 8))
		}
	}

	matrix, e := bmp.GetBlackMatrix()
	if e != nil {
		t.Fatalf("GetBlackRow returns error, %v", e)
	}
	for y := 0; y < matrix.GetHeight(); y++ {
		for x := 0; x < matrix.GetWidth(); x++ {
			if matrix.Get(x, y) != (x < 8) {
				t.Fatalf("BlackMatrix.Get(%v,%v) = %v, expect %v", x, y, matrix.Get(x, y), (x < 8))
			}
		}
	}

	if bmp.IsCropSupported() {
		t.Fatalf("IsCropSupported must false")
	}
	if _, e := bmp.Crop(1, 1, 3, 3); e == nil {
		t.Fatalf("Crop must be error")
	}
	if bmp.IsRotateSupported() {
		t.Fatalf("IsRotateSupported must false")
	}
	if _, e := bmp.RotateCounterClockwise(); e == nil {
		t.Fatalf("RotateCounterClockwise must be error")
	}
	if _, e := bmp.RotateCounterClockwise45(); e == nil {
		t.Fatalf("RotateCounterClockwise45 must be error")
	}

	expectStr := "" +
		"X X X X X X X X                 \n" +
		"X X X X X X X X                 \n" +
		"X X X X X X X X                 \n" +
		"X X X X X X X X                 \n" +
		"X X X X X X X X                 \n" +
		"X X X X X X X X                 \n" +
		"X X X X X X X X                 \n" +
		"X X X X X X X X                 \n" +
		"X X X X X X X X                 \n" +
		"X X X X X X X X                 \n" +
		"X X X X X X X X                 \n" +
		"X X X X X X X X                 \n" +
		"X X X X X X X X                 \n" +
		"X X X X X X X X                 \n" +
		"X X X X X X X X                 \n" +
		"X X X X X X X X                 \n"
	if bmp.String() != expectStr {
		t.Fatalf("\n%v", bmp)
	}
}

type notFoundBinarizer struct {
	testBinarizer
}

func (this *notFoundBinarizer) GetBlackMatrix() (*BitMatrix, error) {
	return nil, NewNotFoundException()
}

type illegalBinarizer struct {
	testBinarizer
}

func (this *illegalBinarizer) GetBlackMatrix() (*BitMatrix, error) {
	return nil, errors.New("IllegalException")
}

func TestBinaryBitmap_NotFound(t *testing.T) {
	var binarizer Binarizer = &notFoundBinarizer{testBinarizer{newTestLuminanceSource(16)}}
	bmp, _ := NewBinaryBitmap(binarizer)

	if _, e := bmp.GetBlackMatrix(); e == nil {
		t.Fatalf("GetBlackMatrix() must be error")
	}
	if s := bmp.String(); s != "" {
		t.Fatalf("Bitmap string = \"%v\", expect \"\"", s)
	}

	bmp, _ = NewBinaryBitmap(&testBinarizer{newCroppableLS(16)})
	if !bmp.IsCropSupported() {
		t.Fatalf("IsCropSupported must true")
	}
	bmp2, e := bmp.Crop(3, 3, 8, 8)
	if e != nil {
		t.Fatalf("Crop returns error, %v", e)
	}
	if w, h := bmp2.GetWidth(), bmp2.GetHeight(); w != 8 || h != 8 {
		t.Fatalf("cropped size = %v,%v, expect 8,8", w, h)
	}

	bmp, _ = NewBinaryBitmap(&testBinarizer{&dummyRotateLS90{newTestLuminanceSource(16)}})
	if !bmp.IsRotateSupported() {
		t.Fatalf("IsRotateSupported must true")
	}
	bmp2, e = bmp.RotateCounterClockwise()
	if e != nil {
		t.Fatalf("RotateCounterClockwise returns error, %v", e)
	}
	src := bmp2.binarizer.GetLuminanceSource()
	if _, ok := src.(*dummyRotateLS90); !ok {
		t.Fatalf("rotated source type must be *dummyRotateLS90, %T", src)
	}
	bmp2, e = bmp.RotateCounterClockwise45()
	if e != nil {
		t.Fatalf("RotateCounterClockwise45 returns error, %v", e)
	}
	src = bmp2.binarizer.GetLuminanceSource()
	if _, ok := src.(*dummyRotateLS45); !ok {
		t.Fatalf("rotated source type must be *dummyRotateLS45, %T", src)
	}
}

func TestBinaryBitmap_Illegal(t *testing.T) {
	var binarizer Binarizer = &illegalBinarizer{testBinarizer{newTestLuminanceSource(16)}}
	bmp, _ := NewBinaryBitmap(binarizer)
	expect := "IllegalException"
	if s := bmp.String(); s != expect {
		t.Fatalf("string = \"%v\", expect \"%v\"", s, expect)
	}
}

func TestBinaryBitmap_SafeConcurrent(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, color.Gray{uint8((x + y) % 256)})
		}
	}

	bmp, err := NewBinaryBitmapFromImage(img)
	if err != nil {
		t.Fatalf("Failed to create binary bitmap: %v", err)
	}

	var wg sync.WaitGroup
	var panicCount int32
	var successCount int32

	numGoroutines := runtime.NumCPU() * 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt32(&panicCount, 1)
					t.Errorf("Goroutine %d panicked: %v", id, r)
				}
			}()

			// Prior to introduction of sync.Once,
			// this call triggered the race condition
			matrix, err := bmp.GetBlackMatrix()
			if err != nil {
				t.Errorf("Goroutine %d got error: %v", id, err)
				return
			}

			// Try to access the matrix - before thread-safe changes, would panic
			for y := 0; y < matrix.GetHeight(); y++ {
				for x := 0; x < matrix.GetWidth(); x++ {
					_ = matrix.Get(x, y)
				}
			}

			atomic.AddInt32(&successCount, 1)
		}(i)
	}

	wg.Wait()

	if panicCount > 0 {
		t.Errorf("Got %d panics accessing shared BinaryBitmap", panicCount)
	}

	t.Logf("All %d goroutines completed successfully (0 panics)", successCount)
}

func TestHybridBinarizer_SafeConcurrent(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, color.Gray{uint8((x + y) % 256)})
		}
	}

	source := NewLuminanceSourceFromImage(img)
	binarizer := NewHybridBinarizer(source)

	var wg sync.WaitGroup
	var panicCount int32
	var successCount int32

	numGoroutines := runtime.NumCPU() * 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt32(&panicCount, 1)
					t.Errorf("Goroutine %d panicked: %v", id, r)
				}
			}()

			matrix, err := binarizer.GetBlackMatrix()
			if err != nil {
				t.Errorf("Goroutine %d got error: %v", id, err)
				return
			}

			for y := 0; y < matrix.GetHeight(); y++ {
				for x := 0; x < matrix.GetWidth(); x++ {
					_ = matrix.Get(x, y)
				}
			}

			atomic.AddInt32(&successCount, 1)
		}(i)
	}

	wg.Wait()

	if panicCount > 0 {
		t.Errorf("Got %d panics accessing shared HybridBinarizer", panicCount)
	}

	t.Logf("All %d goroutines completed successfully with HybridBinarizer (0 panics)", successCount)
}

func TestGetBlackMatrixCalledOnce(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 100, 100))

	t.Run("BinaryBitmap", func(t *testing.T) {
		source := NewLuminanceSourceFromImage(img)
		callCount := int32(0)

		countingBinarizer := &countingBinarizer{
			Binarizer: NewHybridBinarizer(source),
			count:     &callCount,
		}

		bmp, _ := NewBinaryBitmap(countingBinarizer)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = bmp.GetBlackMatrix()
			}()
		}
		wg.Wait()

		if count := atomic.LoadInt32(&callCount); count != 1 {
			t.Errorf("GetBlackMatrix was called %d times, expected 1", count)
		}
	})

	t.Run("HybridBinarizer", func(t *testing.T) {
		source := NewLuminanceSourceFromImage(img)
		binarizer := NewHybridBinarizer(source)

		var wg sync.WaitGroup
		matrices := make([]*BitMatrix, 100)

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				matrix, _ := binarizer.GetBlackMatrix()
				matrices[idx] = matrix
			}(i)
		}
		wg.Wait()

		firstMatrix := matrices[0]
		for i := 1; i < 100; i++ {
			if matrices[i] != firstMatrix {
				t.Errorf("Matrix %d is a different instance than matrix 0", i)
			}
		}
	})
}

// countingBinarizer counts how many times GetBlackMatrix is called
type countingBinarizer struct {
	Binarizer
	count *int32
}

func (c *countingBinarizer) GetBlackMatrix() (*BitMatrix, error) {
	atomic.AddInt32(c.count, 1)
	return c.Binarizer.GetBlackMatrix()
}

// TestRaceDetector illustrates thread-safe implementation leveraging sync.Once
// Run with -race to observe behavior and outcomes
func TestRaceDetector(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 100, 100))
	bmp, _ := NewBinaryBitmapFromImage(img)

	var wg sync.WaitGroup

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = bmp.GetBlackMatrix()
		}()
	}

	wg.Wait()
}

func BenchmarkConcurrentGetBlackMatrix(b *testing.B) {
	img := image.NewGray(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, color.Gray{uint8((x + y) % 256)})
		}
	}

	b.Run("BinaryBitmap", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			bmp, _ := NewBinaryBitmapFromImage(img)
			for pb.Next() {
				_, _ = bmp.GetBlackMatrix()
			}
		})
	})

	b.Run("HybridBinarizer", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			source := NewLuminanceSourceFromImage(img)
			binarizer := NewHybridBinarizer(source)
			for pb.Next() {
				_, _ = binarizer.GetBlackMatrix()
			}
		})
	})
}
