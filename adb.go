// Copyright 2016 The go-vgo Project Developers. See the COPYRIGHT
// file at the top-level directory of this distribution and at
// https://github.com/go-vgo/robotgo/blob/master/LICENSE
//
// Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
// http://www.apache.org/licenses/LICENSE-2.0> or the MIT license
// <LICENSE-MIT or http://opensource.org/licenses/MIT>, at your
// option. This file may not be copied, modified, or distributed
// except according to those terms.

package adb

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/go-vgo/gt/cmd"
	"github.com/vcaesar/adb/internal/mobile"
)

var (
	adb   = "adb "
	adbs  = adb + "shell "
	adbIn = adbs + "input "
)

// RunCmd run the cmd
func RunCmd(in string, l ...string) error {
	var p string
	if len(l) > 0 {
		p = l[0]
	}

	out, e, err := cmd.Run(in)
	if err != nil {
		log.Println(p, out, e, err)
	}

	return err
}

// Devices show the system all adb devices
func Devices() error {
	in := "adb devices"
	return RunCmd(in, "abd devices: ")
}

func Install(app string) error {
	in := adb + "install " + app
	return RunCmd(in, "adb install: ")
}

func Uninstall(app string) error {
	in := adb + "uninstall " + app
	return RunCmd(in, "adb uninstall: ")
}

func Kill(pid string) error {
	in := adbs + "kill " + pid
	return RunCmd(in, "adb kill: ")
}

func Ps() (string, error) {
	in := adbs + "ps"
	out, e, err := cmd.Run(in)
	if err != nil {
		log.Println("ps: ", out, e, err)
	}

	return out, err
}

// RunApp run the android app
func RunApp(appPath string) error {
	in := adbs + "start am start -n " + appPath
	return RunCmd(in, "run app: ")
}

// CloseApp close the android app
func CloseApp(pkgName string) error {
	in := adbs + "am force-stop " + pkgName
	return RunCmd(in, "close app: ")
}

// ActivityApp get the activity apps
func ActivityApp() error {
	in := adbs + "dumpsys activity activities"
	return RunCmd(in, "activity app: ")
}

// ScreenSize get the device screen size
func ScreenSize() (string, error) {
	in := adbs + "wm size "
	out, e, err := cmd.Run(in)
	if err != nil {
		log.Println("screen size: ", out, e, err)
	}

	return out, err
}

// Tap tap the app
func Tap(x, y int) error {
	return Click(x, y)
}

// TapKey tap the key code
func TapKey(key string) error {
	in := adbIn + "keyevent " + key
	return RunCmd(in, "tap key code: ")
}

// TapHome tap the home key
func TapHome() error {
	return TapKey("3")
}

// TapBack tap the back key
func TapBack() error {
	return TapKey("4")
}

func Pull(str string) error {
	in := adb + "pull " + str
	return RunCmd(in, "abd pull: ")
}

func Push(str string) error {
	in := adb + "push " + str
	return RunCmd(in, "adb push: ")
}

// ScreenCap cap the screen
func ScreenCap(path string) error {
	in := adbs + "/system/bin/screencap -p " + path
	return RunCmd(in, "screen cap: ")
}

/*
.______        ______   .______     ______   .___________.  _______   ______
|   _  \      /  __  \  |   _  \   /  __  \  |           | /  _____| /  __  \
|  |_)  |    |  |  |  | |  |_)  | |  |  |  | `---|  |----`|  |  __  |  |  |  |
|      /     |  |  |  | |   _  <  |  |  |  |     |  |     |  | |_ | |  |  |  |
|  |\  \----.|  `--'  | |  |_)  | |  `--'  |     |  |     |  |__| | |  `--'  |
| _| `._____| \______/  |______/   \______/      |__|      \______|  \______/

robotgo compatible API
*/

// Serial selects the device (adb -s) for the robotgo compatible API,
// empty uses the only connected device.
var Serial string

// ErrNotSupported is returned for robotgo calls adb can't do.
var ErrNotSupported = mobile.ErrNotSupported

// execAdb runs adb and returns stdout; replaced in tests.
var execAdb = func(args ...string) ([]byte, error) {
	out, err := exec.Command("adb", args...).Output()
	if ee, ok := err.(*exec.ExitError); ok {
		err = fmt.Errorf("adb %s: %w: %s", strings.Join(args, " "), err, bytes.TrimSpace(ee.Stderr))
	}
	return out, err
}

func run(args ...string) ([]byte, error) {
	if Serial != "" {
		args = append([]string{"-s", Serial}, args...)
	}
	return execAdb(args...)
}

func input(args ...string) error {
	_, err := run(append([]string{"shell", "input"}, args...)...)
	return err
}

type driver struct{}

func (driver) Tap(x, y, n int) error {
	tap := fmt.Sprintf("input tap %d %d", x, y)
	cmds := make([]string, n)
	for i := range cmds {
		cmds[i] = tap
	}
	_, err := run("shell", strings.Join(cmds, " && "))
	return err
}

func (driver) Swipe(x, y, endX, endY, ms int) error {
	return input("swipe", strconv.Itoa(x), strconv.Itoa(y),
		strconv.Itoa(endX), strconv.Itoa(endY), strconv.Itoa(ms))
}

// ScreenSize parses `wm size`, the override size wins over the physical one.
func (driver) ScreenSize() (int, int, error) {
	out, err := run("shell", "wm", "size")
	if err != nil {
		return 0, 0, err
	}

	var w, h int
	for _, line := range strings.Split(string(out), "\n") {
		_, size, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		if _, err := fmt.Sscanf(strings.TrimSpace(size), "%dx%d", &w, &h); err != nil {
			return 0, 0, fmt.Errorf("wm size %q: %w", line, err)
		}
	}
	if w == 0 || h == 0 {
		return 0, 0, fmt.Errorf("wm size: unexpected output %q", out)
	}
	return w, h, nil
}

func (driver) Capture() (image.Image, error) {
	out, err := run("exec-out", "screencap", "-p")
	if err != nil {
		return nil, err
	}
	return png.Decode(bytes.NewReader(out))
}

var robot = &mobile.Robot{Driver: driver{}}

// MilliSleep sleep tm milli second
func MilliSleep(tm int) {
	time.Sleep(time.Duration(tm) * time.Millisecond)
}

// Sleep sleep tm second
func Sleep(tm int) {
	time.Sleep(time.Duration(tm) * time.Second)
}

// Move moves the virtual pointer to x, y, a touch screen has no cursor
func Move(x, y int, displayId ...int) error {
	robot.Move(x, y)
	return nil
}

// MoveRelative moves the virtual pointer by x, y
func MoveRelative(x, y int) error {
	mx, my := robot.Location()
	return Move(mx+x, my+y)
}

// MoveSmooth moves the virtual pointer to x, y
func MoveSmooth(x, y int, args ...interface{}) bool {
	robot.Move(x, y)
	return true
}

// Location returns the virtual pointer position
func Location() (int, int) {
	return robot.Location()
}

// Click taps at the pointer: Click(button string, double bool),
// "right" is a long press; Click(x, y int) taps at x, y.
//
// Examples:
//
//	adb.Click()
//	adb.Click("left", true)
//	adb.Click(100, 200)
func Click(args ...interface{}) error {
	return robot.Click(args...)
}

// MoveClick moves the pointer to x, y and clicks
func MoveClick(x, y int, args ...interface{}) error {
	robot.Move(x, y)
	return robot.Click(args...)
}

// Toggle presses ("down", default) or releases ("up") the touch,
// the release taps, long presses or swipes from the down point.
//
// Examples:
//
//	adb.Toggle("left")
//	adb.Move(100, 900)
//	adb.Toggle("left", "up")
func Toggle(key ...interface{}) error {
	return robot.Toggle(key...)
}

// MouseDown presses the touch at the pointer
func MouseDown(key ...interface{}) error {
	return robot.Toggle("left", "down")
}

// MouseUp releases the touch, see Toggle
func MouseUp(key ...interface{}) error {
	return robot.Toggle("left", "up")
}

// Drag swipes from the pointer to x, y
func Drag(x, y int, args ...string) error {
	return robot.Drag(x, y, mobile.DragMs)
}

// DragSmooth swipes slowly from the pointer to x, y
func DragSmooth(x, y int, args ...interface{}) error {
	return robot.Drag(x, y, mobile.DragSmoothMs)
}

// Swipe swipes x, y to endX, endY in ms milliseconds (default 300)
func Swipe(x, y, endX, endY int, ms ...int) error {
	d := mobile.DragMs
	if len(ms) > 0 {
		d = ms[0]
	}
	return driver{}.Swipe(x, y, endX, endY, d)
}

// Scroll scrolls like a mouse wheel by swiping from the screen center:
// positive y scrolls up, positive x scrolls left, args[0] is the delay ms.
//
// The legacy Scroll(x, y, endX, endY) swipes x, y to endX, endY.
func Scroll(x, y int, args ...int) error {
	if len(args) >= 2 {
		return Swipe(x, y, args[0], args[1])
	}

	if err := robot.Scroll(x, y); err != nil {
		return err
	}
	if len(args) > 0 {
		MilliSleep(args[0])
	}
	return nil
}

// ScrollDir scrolls x units to "up", "down" (default), "left" or "right"
func ScrollDir(x int, direction ...interface{}) error {
	return robot.ScrollDir(x, direction...)
}

// ScrollSmooth scrolls toy units num times: ScrollSmooth(toy, num, tm, tox)
func ScrollSmooth(to int, args ...int) error {
	return robot.ScrollSmooth(to, args...)
}

// GetScreenSize returns the device screen size, 0, 0 on error
func GetScreenSize() (int, int) {
	w, h, err := robot.ScreenSize()
	if err != nil {
		log.Println("get screen size: ", err)
	}
	return w, h
}

// CaptureImg captures the device screen, or the region x, y, w, h
func CaptureImg(args ...int) (image.Image, error) {
	return robot.CaptureImg(args...)
}

// SaveCapture captures the device screen (or region x, y, w, h)
// and saves it to the local path as png or jpeg
func SaveCapture(path string, args ...int) error {
	return robot.SaveCapture(path, args...)
}

// GetPixelColor returns the "rrggbb" color at x, y, "" on error
func GetPixelColor(x, y int, displayId ...int) string {
	c, err := robot.PixelColor(x, y)
	if err != nil {
		log.Println("get pixel color: ", err)
	}
	return c
}

// GetLocationColor returns the color at the pointer
func GetLocationColor(displayId ...int) string {
	return GetPixelColor(robot.Location())
}

// KeyTap taps the key with optional modifiers (Android 13+ for modifiers).
// Keys are robotgo names plus "back", "menu", "power", "camera" and raw
// "KEYCODE_*" names, "home" is the home button.
//
// Examples:
//
//	adb.KeyTap("enter")
//	adb.KeyTap("a", "ctrl")
//	adb.KeyTap("v", []string{"ctrl", "shift"})
func KeyTap(key string, args ...interface{}) error {
	keys := append(mobile.KeyArgs(args), key)
	codes := make([]string, len(keys))
	for i, k := range keys {
		c, err := Keycode(k)
		if err != nil {
			return err
		}
		codes[i] = c
	}

	if len(codes) == 1 {
		return input("keyevent", codes[0])
	}
	return input(append([]string{"keycombination"}, codes...)...)
}

// KeyPress same as KeyTap
func KeyPress(key string, args ...interface{}) error {
	return KeyTap(key, args...)
}

// KeyToggle is not supported, adb input has no separate key down and up
func KeyToggle(key string, args ...interface{}) error {
	return ErrNotSupported
}

// KeyDown is not supported, see KeyToggle
func KeyDown(key string, args ...interface{}) error {
	return ErrNotSupported
}

// KeyUp is not supported, see KeyToggle
func KeyUp(key string, args ...interface{}) error {
	return ErrNotSupported
}

// TypeStr types the string, adb input text supports ASCII only
func TypeStr(str string, args ...int) error {
	s := strings.ReplaceAll(str, " ", "%s")
	return input("text", "'"+strings.ReplaceAll(s, "'", `'\''`)+"'")
}

// Keycode returns the Android keycode of a robotgo key name
func Keycode(key string) (string, error) {
	if strings.HasPrefix(key, "KEYCODE_") {
		return key, nil
	}
	if c, ok := keycodes[strings.ToLower(key)]; ok {
		return strconv.Itoa(c), nil
	}
	return "", fmt.Errorf("unknown key %q", key)
}

// keycodes maps robotgo key names to android.view.KeyEvent codes
var keycodes = func() map[string]int {
	m := map[string]int{
		"home": 3, "back": 4, "call": 5, "endcall": 6,
		"up": 19, "down": 20, "left": 21, "right": 22,
		"audio_vol_up": 24, "audio_vol_down": 25, "power": 26, "camera": 27,
		",": 55, ".": 56, "alt": 57, "lalt": 57, "ralt": 58,
		"shift": 59, "lshift": 59, "rshift": 60, "tab": 61, "space": 62,
		"enter": 66, "return": 66, "backspace": 67, "`": 68, "-": 69, "=": 70,
		"[": 71, "]": 72, "\\": 73, ";": 74, "'": 75, "/": 76, "@": 77,
		"menu": 82, "+": 81, "audio_play": 85, "audio_pause": 85, "audio_stop": 86,
		"audio_next": 87, "audio_prev": 88, "pageup": 92, "pagedown": 93,
		"esc": 111, "escape": 111, "delete": 112,
		"ctrl": 113, "control": 113, "lctrl": 113, "rctrl": 114,
		"capslock": 115, "scrolllock": 116,
		"cmd": 117, "command": 117, "lcmd": 117, "rcmd": 118,
		"printscreen": 120, "insert": 124, "end": 123,
		"audio_mute": 164, "num_lock": 143,
	}
	for i := 0; i < 26; i++ {
		m[string(rune('a'+i))] = 29 + i
	}
	for i := 0; i < 10; i++ {
		m[strconv.Itoa(i)] = 7 + i
	}
	for i := 1; i <= 12; i++ {
		m["f"+strconv.Itoa(i)] = 130 + i
	}
	return m
}()
