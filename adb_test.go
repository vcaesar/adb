package adb

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"strings"
	"testing"
)

// mockAdb records adb calls and answers "wm size" and screencap.
func mockAdb(t *testing.T) *[]string {
	t.Helper()
	var calls []string

	img := image.NewRGBA(image.Rect(0, 0, 1080, 2400))
	img.Set(10, 20, color.RGBA{0xab, 0xcd, 0xef, 0xff})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	old, oldSerial := execAdb, Serial
	execAdb = func(args ...string) ([]byte, error) {
		cmd := strings.Join(args, " ")
		calls = append(calls, cmd)
		switch {
		case strings.HasSuffix(cmd, "shell wm size"):
			return []byte("Physical size: 1440x3200\nOverride size: 1080x2400\n"), nil
		case strings.HasSuffix(cmd, "exec-out screencap -p"):
			return buf.Bytes(), nil
		}
		return nil, nil
	}
	t.Cleanup(func() {
		execAdb, Serial = old, oldSerial
		robot.Move(0, 0)
	})
	return &calls
}

func expect(t *testing.T, calls *[]string, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(*calls, want) {
		t.Fatalf("calls = %q\nwant  %q", *calls, want)
	}
	*calls = nil
}

func TestMouse(t *testing.T) {
	calls := mockAdb(t)
	Serial = "emu-1"

	must(t, Move(100, 200))
	must(t, Click())
	must(t, Click("left", true))
	must(t, Click("right"))
	must(t, Tap(1, 2))
	must(t, MoveRelative(9, 8))
	must(t, MoveClick(5, 6))
	expect(t, calls,
		"-s emu-1 shell input tap 100 200",
		"-s emu-1 shell input tap 100 200 && input tap 100 200",
		"-s emu-1 shell input swipe 100 200 100 200 600",
		"-s emu-1 shell input tap 1 2",
		"-s emu-1 shell input tap 5 6",
	)

	Serial = ""
	if !MoveSmooth(10, 10) {
		t.Fatal("MoveSmooth false")
	}
	must(t, MouseDown())
	must(t, Move(10, 500))
	must(t, MouseUp())
	must(t, Drag(20, 30))
	must(t, DragSmooth(40, 50))
	must(t, Swipe(1, 2, 3, 4, 50))
	must(t, Toggle("left"))
	must(t, Toggle("left", "up"))
	expect(t, calls,
		"shell input swipe 10 10 10 500 100",
		"shell input swipe 10 500 20 30 300",
		"shell input swipe 20 30 40 50 800",
		"shell input swipe 1 2 3 4 50",
		"shell input tap 40 50",
	)
	if x, y := Location(); x != 40 || y != 50 {
		t.Fatalf("location = %d, %d", x, y)
	}
}

func TestScroll(t *testing.T) {
	calls := mockAdb(t)

	must(t, Scroll(0, 2))
	must(t, Scroll(0, -1, 1))
	must(t, ScrollDir(1, "left"))
	must(t, ScrollSmooth(1, 1, 0))
	must(t, Scroll(1, 2, 3, 4)) // legacy swipe
	expect(t, calls,
		"shell wm size", "shell input swipe 540 1200 540 1440 200",
		"shell wm size", "shell input swipe 540 1200 540 1080 200",
		"shell wm size", "shell input swipe 540 1200 594 1200 200",
		"shell wm size", "shell input swipe 540 1200 540 1320 200",
		"shell input swipe 1 2 3 4 300",
	)
}

func TestScreen(t *testing.T) {
	calls := mockAdb(t)

	if w, h := GetScreenSize(); w != 1080 || h != 2400 {
		t.Fatalf("size = %d, %d", w, h)
	}
	if c := GetPixelColor(10, 20); c != "abcdef" {
		t.Fatalf("color = %q", c)
	}
	must(t, Move(10, 20))
	if c := GetLocationColor(); c != "abcdef" {
		t.Fatalf("location color = %q", c)
	}
	img, err := CaptureImg(0, 0, 50, 60)
	must(t, err)
	if img.Bounds().Dx() != 50 || img.Bounds().Dy() != 60 {
		t.Fatalf("crop = %v", img.Bounds())
	}
	must(t, SaveCapture(t.TempDir()+"/s.png"))
	*calls = nil

	execAdb = func(args ...string) ([]byte, error) { return []byte("garbage"), nil }
	if w, h := GetScreenSize(); w != 0 || h != 0 {
		t.Fatalf("size = %d, %d, want 0, 0", w, h)
	}
	execAdb = func(args ...string) ([]byte, error) { return []byte("Physical size: nope"), nil }
	if _, _, err := (driver{}).ScreenSize(); err == nil {
		t.Fatal("want parse error")
	}
	execAdb = func(args ...string) ([]byte, error) { return nil, errors.New("no device") }
	if GetPixelColor(0, 0) != "" {
		t.Fatal("want empty color")
	}
	if _, err := CaptureImg(); err == nil {
		t.Fatal("want capture error")
	}
}

func TestKeys(t *testing.T) {
	calls := mockAdb(t)

	must(t, KeyTap("enter"))
	must(t, KeyTap("A"))
	must(t, KeyTap("home"))
	must(t, KeyPress("KEYCODE_BACK"))
	must(t, KeyTap("v", "ctrl"))
	must(t, KeyTap("f12", []string{"cmd", "shift"}, 1234))
	must(t, TypeStr("it's a test"))
	expect(t, calls,
		"shell input keyevent 66",
		"shell input keyevent 29",
		"shell input keyevent 3",
		"shell input keyevent KEYCODE_BACK",
		"shell input keycombination 113 50",
		"shell input keycombination 117 59 142",
		`shell input text 'it'\''s%sa%stest'`,
	)

	if KeyTap("nope") == nil || KeyTap("a", "hyper") == nil {
		t.Fatal("want unknown key errors")
	}
	for _, err := range []error{KeyToggle("a"), KeyDown("a"), KeyUp("a")} {
		if !errors.Is(err, ErrNotSupported) {
			t.Fatalf("err = %v", err)
		}
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
