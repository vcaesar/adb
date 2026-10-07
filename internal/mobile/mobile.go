// Package mobile maps the robotgo mouse and screen API onto touch devices.
//
// A touch screen has no cursor, so Robot keeps a virtual pointer:
// Move only updates it, Click taps at it, and Toggle down/up turns
// into a press, long press or swipe from the down point to the up point.
package mobile

import (
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrNotSupported is returned for robotgo calls a touch device can't do.
var ErrNotSupported = errors.New("not supported on this device")

// Gesture durations in milliseconds.
const (
	LongPressMs  = 600
	DragMs       = 300
	DragSmoothMs = 800
	ScrollMs     = 200
)

// Driver is the device backend (adb, WebDriverAgent).
type Driver interface {
	// Tap taps n times at x, y.
	Tap(x, y, n int) error
	// Swipe drags from x, y to endX, endY over ms milliseconds;
	// the same start and end point is a long press.
	Swipe(x, y, endX, endY, ms int) error
	// ScreenSize returns the touch coordinate space size.
	ScreenSize() (int, int, error)
	// Capture returns a full screenshot.
	Capture() (image.Image, error)
}

// Robot is a robotgo-style virtual pointer over a Driver.
type Robot struct {
	Driver

	x, y         int
	down         bool
	downX, downY int
	downAt       time.Time
}

// Move moves the virtual pointer.
func (r *Robot) Move(x, y int) {
	r.x, r.y = x, y
}

// Location returns the virtual pointer position.
func (r *Robot) Location() (int, int) {
	return r.x, r.y
}

// Click taps at the pointer, robotgo style: Click(button string, double bool).
// "right" is a long press. Click(x, y int) taps at x, y.
func (r *Robot) Click(args ...interface{}) error {
	if len(args) == 2 {
		x, okX := args[0].(int)
		y, okY := args[1].(int)
		if okX && okY {
			r.Move(x, y)
			return r.Tap(x, y, 1)
		}
	}

	button, double := "left", false
	if len(args) > 0 {
		s, ok := args[0].(string)
		if !ok {
			return errors.New("first argument must be button string")
		}
		button = s
	}
	if len(args) > 1 {
		b, ok := args[1].(bool)
		if !ok {
			return errors.New("second argument must be bool indicating double click")
		}
		double = b
	}

	if button == "right" {
		return r.Swipe(r.x, r.y, r.x, r.y, LongPressMs)
	}
	if double {
		return r.Tap(r.x, r.y, 2)
	}
	return r.Tap(r.x, r.y, 1)
}

// Toggle presses ("down", default) or releases ("up") the touch, robotgo
// style: Toggle(button, "up"). The release performs the gesture from the
// down point to the current pointer, held for the time between the two.
func (r *Robot) Toggle(args ...interface{}) error {
	if len(args) < 2 || args[1] != "up" {
		r.down = true
		r.downX, r.downY, r.downAt = r.x, r.y, time.Now()
		return nil
	}
	if !r.down {
		return nil
	}

	r.down = false
	ms := int(time.Since(r.downAt) / time.Millisecond)
	if r.downX == r.x && r.downY == r.y && ms < LongPressMs {
		return r.Tap(r.x, r.y, 1)
	}
	if ms < 100 {
		ms = 100
	}
	return r.Swipe(r.downX, r.downY, r.x, r.y, ms)
}

// Drag swipes from the pointer to x, y over ms and moves the pointer there.
func (r *Robot) Drag(x, y, ms int) error {
	fx, fy := r.x, r.y
	r.Move(x, y)
	return r.Swipe(fx, fy, x, y, ms)
}

// Scroll swipes from the screen center like a mouse wheel: positive y
// scrolls up, positive x scrolls left, one unit is 1/20 of the screen.
func (r *Robot) Scroll(x, y int) error {
	w, h, err := r.ScreenSize()
	if err != nil {
		return err
	}

	cx, cy := w/2, h/2
	ex := clamp(cx+x*w/20, 1, w-1)
	ey := clamp(cy+y*h/20, 1, h-1)
	return r.Swipe(cx, cy, ex, ey, ScrollMs)
}

// ScrollDir scrolls n units to "up", "down" (default), "left" or "right".
func (r *Robot) ScrollDir(n int, direction ...interface{}) error {
	d := "down"
	if len(direction) > 0 {
		s, ok := direction[0].(string)
		if !ok {
			return fmt.Errorf("unknown scroll direction: %v", direction[0])
		}
		d = s
	}

	switch d {
	case "down":
		return r.Scroll(0, -n)
	case "up":
		return r.Scroll(0, n)
	case "left":
		return r.Scroll(n, 0)
	case "right":
		return r.Scroll(-n, 0)
	}
	return fmt.Errorf("unknown scroll direction: %v", d)
}

// ScrollSmooth scrolls toy units num times (default 5), sleeping tm
// milliseconds (default 100) between: ScrollSmooth(toy, num, tm, tox).
func (r *Robot) ScrollSmooth(to int, args ...int) error {
	num, tm, tox := 5, 100, 0
	if len(args) > 0 {
		num = args[0]
	}
	if len(args) > 1 {
		tm = args[1]
	}
	if len(args) > 2 {
		tox = args[2]
	}

	for i := 0; i < num; i++ {
		if err := r.Scroll(tox, to); err != nil {
			return err
		}
		time.Sleep(time.Duration(tm) * time.Millisecond)
	}
	return nil
}

// CaptureImg captures the screen, or the region x, y, w, h given in
// touch coordinates.
func (r *Robot) CaptureImg(args ...int) (image.Image, error) {
	img, err := r.Capture()
	if err != nil || len(args) < 4 {
		return img, err
	}

	s, err := r.scale(img)
	if err != nil {
		return nil, err
	}
	rect := image.Rect(
		int(float64(args[0])*s), int(float64(args[1])*s),
		int(float64(args[0]+args[2])*s), int(float64(args[1]+args[3])*s),
	).Add(img.Bounds().Min)

	sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok {
		return nil, fmt.Errorf("capture: can't crop %T", img)
	}
	return sub.SubImage(rect), nil
}

// SaveCapture captures the screen (or region) and saves it as png or jpeg
// by the path extension.
func (r *Robot) SaveCapture(path string, args ...int) error {
	img, err := r.CaptureImg(args...)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		err = jpeg.Encode(f, img, nil)
	default:
		err = png.Encode(f, img)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// PixelColor returns the "rrggbb" hex color at x, y in touch coordinates.
func (r *Robot) PixelColor(x, y int) (string, error) {
	img, err := r.Capture()
	if err != nil {
		return "", err
	}
	s, err := r.scale(img)
	if err != nil {
		return "", err
	}

	o := img.Bounds().Min
	cr, cg, cb, _ := img.At(o.X+int(float64(x)*s), o.Y+int(float64(y)*s)).RGBA()
	return fmt.Sprintf("%02x%02x%02x", cr>>8, cg>>8, cb>>8), nil
}

// scale returns screenshot pixels per touch unit.
func (r *Robot) scale(img image.Image) (float64, error) {
	w, h, err := r.ScreenSize()
	if err != nil {
		return 0, err
	}
	if w <= 0 || h <= 0 {
		return 0, fmt.Errorf("invalid screen size %dx%d", w, h)
	}

	b := img.Bounds()
	if (b.Dx() > b.Dy()) != (w > h) {
		w = h
	}
	return float64(b.Dx()) / float64(w), nil
}

// KeyArgs returns the modifier keys of robotgo KeyTap args, which may be
// strings or []string; a pid int is ignored.
func KeyArgs(args []interface{}) []string {
	var mods []string
	for _, a := range args {
		switch v := a.(type) {
		case string:
			mods = append(mods, v)
		case []string:
			mods = append(mods, v...)
		}
	}
	return mods
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
