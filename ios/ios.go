// Package ios is a robotgo compatible API for iOS devices,
// driven by WebDriverAgent (https://github.com/appium/WebDriverAgent).
//
// Start WebDriverAgent on the device and forward its port first, e.g.
//
//	ios runwda && ios forward 8100 8100  // github.com/danielpaulus/go-ios
//
// Coordinates are WDA points, screenshots are device pixels.
package ios

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/vcaesar/adb/internal/mobile"
)

// URL is the WebDriverAgent server address.
var URL = "http://127.0.0.1:8100"

// Client is the HTTP client used to call WebDriverAgent.
var Client = &http.Client{Timeout: 60 * time.Second}

// ErrNotSupported is returned for robotgo calls iOS can't do.
var ErrNotSupported = mobile.ErrNotSupported

// Error is a WebDriverAgent error response.
type Error struct {
	Err     string `json:"error"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return "wda: " + e.Err + ": " + e.Message
}

var sessionID string

// call sends a WDA request and decodes the response "value" into out.
func call(method, path string, body, out interface{}) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, strings.TrimRight(URL, "/")+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var r struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return fmt.Errorf("wda %s %s: status %d: %w", method, path, resp.StatusCode, err)
	}
	if resp.StatusCode >= 300 {
		e := &Error{}
		if err := json.Unmarshal(r.Value, e); err != nil || e.Err == "" {
			return fmt.Errorf("wda %s %s: status %d: %s", method, path, resp.StatusCode, r.Value)
		}
		return e
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(r.Value, out)
}

// sessionCall calls a session route, creating the session on demand
// and once more if WDA restarted.
func sessionCall(method, path string, body, out interface{}) error {
	for i := 0; ; i++ {
		if sessionID == "" {
			var s struct {
				SessionID string `json:"sessionId"`
			}
			caps := map[string]interface{}{"capabilities": map[string]interface{}{}}
			if err := call("POST", "/session", caps, &s); err != nil {
				return err
			}
			sessionID = s.SessionID
		}

		err := call(method, "/session/"+sessionID+path, body, out)
		var e *Error
		if i == 0 && errors.As(err, &e) && e.Err == "invalid session id" {
			sessionID = ""
			continue
		}
		return err
	}
}

func move(x, y, ms int) map[string]interface{} {
	return map[string]interface{}{
		"type": "pointerMove", "duration": ms, "x": x, "y": y, "origin": "viewport"}
}

func pause(ms int) map[string]interface{} {
	return map[string]interface{}{"type": "pause", "duration": ms}
}

var (
	pointerDown = map[string]interface{}{"type": "pointerDown", "button": 0}
	pointerUp   = map[string]interface{}{"type": "pointerUp", "button": 0}
)

// touch performs W3C touch actions.
func touch(steps ...map[string]interface{}) error {
	body := map[string]interface{}{"actions": []interface{}{map[string]interface{}{
		"type": "pointer", "id": "finger1",
		"parameters": map[string]interface{}{"pointerType": "touch"},
		"actions":    steps,
	}}}
	return sessionCall("POST", "/actions", body, nil)
}

type driver struct{}

func (driver) Tap(x, y, n int) error {
	steps := []map[string]interface{}{move(x, y, 0)}
	for i := 0; i < n; i++ {
		if i > 0 {
			steps = append(steps, pause(50))
		}
		steps = append(steps, pointerDown, pause(50), pointerUp)
	}
	return touch(steps...)
}

func (driver) Swipe(x, y, endX, endY, ms int) error {
	if x == endX && y == endY {
		return touch(move(x, y, 0), pointerDown, pause(ms), pointerUp)
	}
	return touch(move(x, y, 0), pointerDown, move(endX, endY, ms), pointerUp)
}

func (driver) ScreenSize() (int, int, error) {
	var s struct {
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	}
	if err := sessionCall("GET", "/window/size", nil, &s); err != nil {
		return 0, 0, err
	}
	return int(s.Width), int(s.Height), nil
}

func (driver) Capture() (image.Image, error) {
	var b64 string
	if err := call("GET", "/screenshot", nil, &b64); err != nil {
		return nil, err
	}
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	return png.Decode(bytes.NewReader(b))
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
func Click(args ...interface{}) error {
	return robot.Click(args...)
}

// Tap taps at x, y
func Tap(x, y int) error {
	return robot.Click(x, y)
}

// MoveClick moves the pointer to x, y and clicks
func MoveClick(x, y int, args ...interface{}) error {
	robot.Move(x, y)
	return robot.Click(args...)
}

// Toggle presses ("down", default) or releases ("up") the touch,
// the release taps, long presses or swipes from the down point.
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
func Scroll(x, y int, args ...int) error {
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

// GetScreenSize returns the screen size in points, 0, 0 on error
func GetScreenSize() (int, int) {
	w, h, err := robot.ScreenSize()
	if err != nil {
		log.Println("get screen size: ", err)
	}
	return w, h
}

// CaptureImg captures the screen, or the region x, y, w, h in points
func CaptureImg(args ...int) (image.Image, error) {
	return robot.CaptureImg(args...)
}

// SaveCapture captures the screen (or region x, y, w, h)
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

// buttons maps key names to WDA hardware button names
var buttons = map[string]string{
	"home":           "home",
	"audio_vol_up":   "volumeUp",
	"audio_vol_down": "volumeDown",
}

// keys maps robotgo key names to XCUIKeyboardKey names
var keys = map[string]string{
	"enter": "XCUIKeyboardKeyReturn", "return": "XCUIKeyboardKeyReturn",
	"backspace": "XCUIKeyboardKeyDelete", "delete": "XCUIKeyboardKeyForwardDelete",
	"tab": "XCUIKeyboardKeyTab", "space": "XCUIKeyboardKeySpace",
	"esc": "XCUIKeyboardKeyEscape", "escape": "XCUIKeyboardKeyEscape",
	"up": "XCUIKeyboardKeyUpArrow", "down": "XCUIKeyboardKeyDownArrow",
	"left": "XCUIKeyboardKeyLeftArrow", "right": "XCUIKeyboardKeyRightArrow",
	"end": "XCUIKeyboardKeyEnd", "pageup": "XCUIKeyboardKeyPageUp",
	"pagedown": "XCUIKeyboardKeyPageDown",
}

// typed maps keys to text for /wda/keys, which works on all iOS versions
var typed = map[string]string{
	"enter": "\n", "return": "\n", "backspace": "\b", "tab": "\t", "space": " ",
}

// modifiers maps robotgo modifiers to XCUIKeyModifierFlags
var modifiers = map[string]int{
	"capslock": 1 << 0, "shift": 1 << 1, "lshift": 1 << 1, "rshift": 1 << 1,
	"ctrl": 1 << 2, "control": 1 << 2, "lctrl": 1 << 2, "rctrl": 1 << 2,
	"alt": 1 << 3, "lalt": 1 << 3, "ralt": 1 << 3,
	"cmd": 1 << 4, "command": 1 << 4, "lcmd": 1 << 4, "rcmd": 1 << 4,
}

// KeyTap taps the key, "home", "audio_vol_up" and "audio_vol_down"
// press the hardware buttons. Modifiers and keys other than enter,
// backspace, tab, space and characters need iPadOS 17+ (Xcode 15).
//
// Examples:
//
//	ios.KeyTap("enter")
//	ios.KeyTap("home")
//	ios.KeyTap("a", "cmd")
func KeyTap(key string, args ...interface{}) error {
	k := strings.ToLower(key)
	if len([]rune(key)) == 1 {
		k = key
	}
	if b, ok := buttons[k]; ok {
		return sessionCall("POST", "/wda/pressButton", map[string]interface{}{"name": b}, nil)
	}

	mods := mobile.KeyArgs(args)
	if t, ok := typed[k]; ok && len(mods) == 0 {
		return TypeStr(t)
	}
	if len([]rune(k)) == 1 && len(mods) == 0 {
		return TypeStr(k)
	}

	flags := 0
	for _, m := range mods {
		f, ok := modifiers[strings.ToLower(m)]
		if !ok {
			return fmt.Errorf("unknown modifier %q", m)
		}
		flags |= f
	}
	if name, ok := keys[k]; ok {
		k = name
	} else if len([]rune(k)) != 1 {
		return fmt.Errorf("unknown key %q", key)
	}

	body := map[string]interface{}{"keys": []interface{}{
		map[string]interface{}{"key": k, "modifierFlags": flags}}}
	return sessionCall("POST", "/wda/element/0/keyboardInput", body, nil)
}

// KeyPress same as KeyTap
func KeyPress(key string, args ...interface{}) error {
	return KeyTap(key, args...)
}

// KeyToggle is not supported, WDA has no separate key down and up
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

// TypeStr types the string (UTF-8) into the focused element
func TypeStr(str string, args ...int) error {
	return sessionCall("POST", "/wda/keys", map[string]interface{}{"value": []string{str}}, nil)
}

// WriteAll writes the text to the device pasteboard
func WriteAll(text string) error {
	body := map[string]interface{}{
		"content":     base64.StdEncoding.EncodeToString([]byte(text)),
		"contentType": "plaintext",
	}
	return sessionCall("POST", "/wda/setPasteboard", body, nil)
}

// ReadAll reads the text of the device pasteboard,
// iOS only allows it while WDA is the foreground app
func ReadAll() (string, error) {
	var b64 string
	body := map[string]interface{}{"contentType": "plaintext"}
	if err := sessionCall("POST", "/wda/getPasteboard", body, &b64); err != nil {
		return "", err
	}
	b, err := base64.StdEncoding.DecodeString(b64)
	return string(b), err
}

// TapHome goes to the home screen
func TapHome() error {
	return call("POST", "/wda/homescreen", nil, nil)
}

// RunApp launches the app by bundle id
func RunApp(bundleID string) error {
	return sessionCall("POST", "/wda/apps/launch", map[string]interface{}{"bundleId": bundleID}, nil)
}

// CloseApp terminates the app by bundle id
func CloseApp(bundleID string) error {
	return sessionCall("POST", "/wda/apps/terminate", map[string]interface{}{"bundleId": bundleID}, nil)
}

// ActiveApp returns the bundle id of the foreground app
func ActiveApp() (string, error) {
	var a struct {
		BundleID string `json:"bundleId"`
	}
	err := call("GET", "/wda/activeAppInfo", nil, &a)
	return a.BundleID, err
}
