package mobile

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type fake struct {
	calls []string
	w, h  int
	img   image.Image
}

func (f *fake) Tap(x, y, n int) error {
	f.calls = append(f.calls, fmt.Sprintf("tap %d %d %d", x, y, n))
	return nil
}

func (f *fake) Swipe(x, y, endX, endY, ms int) error {
	f.calls = append(f.calls, fmt.Sprintf("swipe %d %d %d %d %d", x, y, endX, endY, ms))
	return nil
}

func (f *fake) ScreenSize() (int, int, error) { return f.w, f.h, nil }

func (f *fake) Capture() (image.Image, error) { return f.img, nil }

func newRobot() (*Robot, *fake) {
	f := &fake{w: 100, h: 200}
	return &Robot{Driver: f}, f
}

func expect(t *testing.T, f *fake, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("calls = %q, want %q", f.calls, want)
	}
}

func TestClick(t *testing.T) {
	r, f := newRobot()
	r.Move(10, 20)
	for _, args := range [][]interface{}{{}, {"left", true}, {"right"}, {30, 40}} {
		if err := r.Click(args...); err != nil {
			t.Fatal(err)
		}
	}
	expect(t, f, "tap 10 20 1", "tap 10 20 2", "swipe 10 20 10 20 600", "tap 30 40 1")
	if x, y := r.Location(); x != 30 || y != 40 {
		t.Fatalf("location = %d, %d", x, y)
	}

	if r.Click(1) == nil || r.Click("left", "x") == nil {
		t.Fatal("want argument errors")
	}
}

func TestToggle(t *testing.T) {
	r, f := newRobot()
	if err := r.Toggle("left", "up"); err != nil {
		t.Fatal(err)
	}
	expect(t, f)

	r.Move(5, 5)
	if err := r.Toggle("left"); err != nil {
		t.Fatal(err)
	}
	if err := r.Toggle("left", "up"); err != nil {
		t.Fatal(err)
	}
	expect(t, f, "tap 5 5 1")

	f.calls = nil
	if err := r.Toggle(); err != nil {
		t.Fatal(err)
	}
	r.Move(50, 60)
	if err := r.Toggle("left", "up"); err != nil {
		t.Fatal(err)
	}
	expect(t, f, "swipe 5 5 50 60 100")

	f.calls = nil
	if err := r.Toggle("left", "down"); err != nil {
		t.Fatal(err)
	}
	r.downAt = time.Now().Add(-time.Second)
	if err := r.Toggle("left", "up"); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 || f.calls[0][:18] != "swipe 50 60 50 60 " {
		t.Fatalf("long press calls = %q", f.calls)
	}
}

func TestDragScroll(t *testing.T) {
	r, f := newRobot()
	r.Move(1, 2)
	if err := r.Drag(3, 4, DragMs); err != nil {
		t.Fatal(err)
	}
	if err := r.Scroll(0, 2); err != nil {
		t.Fatal(err)
	}
	if err := r.ScrollDir(100, "down"); err != nil {
		t.Fatal(err)
	}
	if err := r.ScrollDir(1, "left"); err != nil {
		t.Fatal(err)
	}
	if err := r.ScrollDir(1, "right"); err != nil {
		t.Fatal(err)
	}
	if err := r.ScrollSmooth(1, 2, 0); err != nil {
		t.Fatal(err)
	}
	expect(t, f,
		"swipe 1 2 3 4 300",
		"swipe 50 100 50 120 200",
		"swipe 50 100 50 1 200",
		"swipe 50 100 55 100 200",
		"swipe 50 100 45 100 200",
		"swipe 50 100 50 110 200",
		"swipe 50 100 50 110 200",
	)
	if r.ScrollDir(1, "diagonal") == nil || r.ScrollDir(1, 2) == nil {
		t.Fatal("want direction errors")
	}
}

func testImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(30, 60, color.RGBA{0x12, 0x34, 0x56, 0xff})
	return img
}

func TestCapture(t *testing.T) {
	r, f := newRobot()
	f.img = testImage(300, 600) // 3x scale, like an iOS screenshot

	img, err := r.CaptureImg(10, 20, 30, 40)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b != image.Rect(30, 60, 120, 180) {
		t.Fatalf("crop = %v", b)
	}

	c, err := r.PixelColor(10, 20)
	if err != nil || c != "123456" {
		t.Fatalf("color = %q, %v", c, err)
	}

	// landscape screenshot of a portrait size
	f.img = testImage(600, 300)
	if s, err := r.scale(f.img); err != nil || s != 3 {
		t.Fatalf("scale = %v, %v", s, err)
	}

	f.w = 0
	if _, err := r.PixelColor(0, 0); err == nil {
		t.Fatal("want invalid size error")
	}
}

func TestSaveCapture(t *testing.T) {
	r, f := newRobot()
	f.img = testImage(100, 200)
	dir := t.TempDir()

	for name, decode := range map[string]func(*os.File) (image.Image, error){
		"a.png": func(f *os.File) (image.Image, error) { return png.Decode(f) },
		"a.jpg": func(f *os.File) (image.Image, error) { return jpeg.Decode(f) },
	} {
		path := filepath.Join(dir, name)
		if err := r.SaveCapture(path, 0, 0, 10, 10); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		img, err := decode(file)
		file.Close()
		if err != nil || img.Bounds().Dx() != 10 {
			t.Fatalf("%s: %v %v", name, img.Bounds(), err)
		}
	}

	if r.SaveCapture(filepath.Join(dir, "missing", "a.png")) == nil {
		t.Fatal("want create error")
	}
}

func TestKeyArgs(t *testing.T) {
	got := KeyArgs([]interface{}{"ctrl", []string{"alt", "shift"}, 123})
	if !reflect.DeepEqual(got, []string{"ctrl", "alt", "shift"}) {
		t.Fatalf("KeyArgs = %q", got)
	}
}
