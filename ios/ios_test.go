package ios

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type req struct {
	Method, Path string
	Body         map[string]interface{}
}

// fakeWDA serves a minimal WebDriverAgent: 390x844 points, 3x screenshot.
type fakeWDA struct {
	mu         sync.Mutex
	reqs       []req
	pasteboard string
	expired    bool
}

func (f *fakeWDA) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body = nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = append(f.reqs, req{r.Method, r.URL.Path, body})

	reply := func(status int, v interface{}) {
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{"value": v}); err != nil {
			panic(err)
		}
	}

	p := r.URL.Path
	switch {
	case p == "/session":
		reply(200, map[string]interface{}{"sessionId": "s1"})
	case f.expired && strings.HasPrefix(p, "/session/"):
		f.expired = false
		reply(404, map[string]interface{}{"error": "invalid session id", "message": "gone"})
	case strings.HasSuffix(p, "/window/size"):
		reply(200, map[string]interface{}{"width": 390.0, "height": 844.0})
	case p == "/screenshot":
		img := image.NewRGBA(image.Rect(0, 0, 1170, 2532))
		img.Set(30, 60, color.RGBA{0x12, 0x34, 0x56, 0xff})
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			panic(err)
		}
		reply(200, base64.StdEncoding.EncodeToString(buf.Bytes()))
	case strings.HasSuffix(p, "/wda/setPasteboard"):
		f.pasteboard = body["content"].(string)
		reply(200, nil)
	case strings.HasSuffix(p, "/wda/getPasteboard"):
		reply(200, f.pasteboard)
	case p == "/wda/activeAppInfo":
		reply(200, map[string]interface{}{"bundleId": "com.apple.Preferences"})
	case strings.HasSuffix(p, "/wda/element/0/keyboardInput"):
		reply(500, map[string]interface{}{"error": "unsupported operation", "message": "typeKey API is only supported since Xcode15"})
	case p == "/broken":
		w.WriteHeader(500)
	default:
		reply(200, nil)
	}
}

// take returns and clears the recorded requests, dropping session creation.
func (f *fakeWDA) take() []req {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []req
	for _, r := range f.reqs {
		if r.Path != "/session" {
			out = append(out, r)
		}
	}
	f.reqs = nil
	return out
}

func setup(t *testing.T) *fakeWDA {
	t.Helper()
	f := &fakeWDA{}
	srv := httptest.NewServer(f)
	old := URL
	URL, sessionID = srv.URL+"/", ""
	t.Cleanup(func() {
		srv.Close()
		URL, sessionID = old, ""
		robot.Move(0, 0)
	})
	return f
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// js round-trips v through JSON so it compares with decoded bodies.
func js(t *testing.T, v interface{}) map[string]interface{} {
	t.Helper()
	b, err := json.Marshal(v)
	must(t, err)
	var m map[string]interface{}
	must(t, json.Unmarshal(b, &m))
	return m
}

func actions(t *testing.T, steps ...map[string]interface{}) map[string]interface{} {
	return js(t, map[string]interface{}{"actions": []interface{}{map[string]interface{}{
		"type": "pointer", "id": "finger1",
		"parameters": map[string]interface{}{"pointerType": "touch"},
		"actions":    steps,
	}}})
}

func expect(t *testing.T, got []req, want ...req) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d requests %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Fatalf("request %d:\n got %+v\nwant %+v", i, got[i], want[i])
		}
	}
}

func TestTouch(t *testing.T) {
	f := setup(t)

	must(t, Move(10, 20))
	must(t, Click())
	must(t, Click("left", true))
	must(t, Click("right"))
	must(t, Tap(1, 2))
	must(t, Swipe(1, 2, 3, 4))

	const a = "/session/s1/actions"
	expect(t, f.take(),
		req{"POST", a, actions(t, move(10, 20, 0), pointerDown, pause(50), pointerUp)},
		req{"POST", a, actions(t, move(10, 20, 0), pointerDown, pause(50), pointerUp,
			pause(50), pointerDown, pause(50), pointerUp)},
		req{"POST", a, actions(t, move(10, 20, 0), pointerDown, pause(600), pointerUp)},
		req{"POST", a, actions(t, move(1, 2, 0), pointerDown, pause(50), pointerUp)},
		req{"POST", a, actions(t, move(1, 2, 0), pointerDown, move(3, 4, 300), pointerUp)},
	)

	must(t, MoveRelative(1, 1))
	if !MoveSmooth(5, 5) {
		t.Fatal("MoveSmooth false")
	}
	must(t, MouseDown())
	must(t, Move(5, 400))
	must(t, MouseUp())
	must(t, Toggle("left"))
	must(t, Toggle("left", "up"))
	must(t, Drag(50, 60))
	must(t, DragSmooth(70, 80))
	must(t, MoveClick(9, 9))
	expect(t, f.take(),
		req{"POST", a, actions(t, move(5, 5, 0), pointerDown, move(5, 400, 100), pointerUp)},
		req{"POST", a, actions(t, move(5, 400, 0), pointerDown, pause(50), pointerUp)},
		req{"POST", a, actions(t, move(5, 400, 0), pointerDown, move(50, 60, 300), pointerUp)},
		req{"POST", a, actions(t, move(50, 60, 0), pointerDown, move(70, 80, 800), pointerUp)},
		req{"POST", a, actions(t, move(9, 9, 0), pointerDown, pause(50), pointerUp)},
	)
	if x, y := Location(); x != 9 || y != 9 {
		t.Fatalf("location = %d, %d", x, y)
	}
}

func TestScroll(t *testing.T) {
	f := setup(t)

	must(t, Scroll(0, 2, 1))
	must(t, ScrollDir(1, "up"))
	must(t, ScrollSmooth(-1, 1, 0))
	const a = "/session/s1/actions"
	size := req{"GET", "/session/s1/window/size", nil}
	expect(t, f.take(),
		size, req{"POST", a, actions(t, move(195, 422, 0), pointerDown, move(195, 506, 200), pointerUp)},
		size, req{"POST", a, actions(t, move(195, 422, 0), pointerDown, move(195, 464, 200), pointerUp)},
		size, req{"POST", a, actions(t, move(195, 422, 0), pointerDown, move(195, 380, 200), pointerUp)},
	)
}

func TestScreen(t *testing.T) {
	setup(t)

	if w, h := GetScreenSize(); w != 390 || h != 844 {
		t.Fatalf("size = %d, %d", w, h)
	}
	if c := GetPixelColor(10, 20); c != "123456" {
		t.Fatalf("color = %q", c)
	}
	must(t, Move(10, 20))
	if c := GetLocationColor(); c != "123456" {
		t.Fatalf("location color = %q", c)
	}
	img, err := CaptureImg(0, 0, 100, 200)
	must(t, err)
	if b := img.Bounds(); b.Dx() != 300 || b.Dy() != 600 {
		t.Fatalf("crop = %v", b)
	}
	must(t, SaveCapture(t.TempDir()+"/s.jpg", 0, 0, 10, 10))
}

func TestKeys(t *testing.T) {
	f := setup(t)

	must(t, KeyTap("enter"))
	must(t, KeyTap("A"))
	must(t, KeyPress("home"))
	must(t, KeyTap("audio_vol_up"))
	must(t, TypeStr("héllo"))
	keys := func(s string) req {
		return req{"POST", "/session/s1/wda/keys", js(t, map[string]interface{}{"value": []string{s}})}
	}
	button := func(s string) req {
		return req{"POST", "/session/s1/wda/pressButton", map[string]interface{}{"name": s}}
	}
	expect(t, f.take(), keys("\n"), keys("A"), button("home"), button("volumeUp"), keys("héllo"))

	var e *Error
	if err := KeyTap("a", "cmd", []string{"shift"}); !errors.As(err, &e) || e.Err != "unsupported operation" {
		t.Fatalf("err = %v", err)
	}
	if err := KeyTap("left"); !errors.As(err, &e) {
		t.Fatalf("err = %v", err)
	}
	input := func(k string, flags float64) req {
		return req{"POST", "/session/s1/wda/element/0/keyboardInput", map[string]interface{}{
			"keys": []interface{}{map[string]interface{}{"key": k, "modifierFlags": flags}}}}
	}
	expect(t, f.take(), input("a", 18), input("XCUIKeyboardKeyLeftArrow", 0))

	if KeyTap("nope") == nil || KeyTap("a", "hyper") == nil {
		t.Fatal("want unknown key errors")
	}
	for _, err := range []error{KeyToggle("a"), KeyDown("a"), KeyUp("a")} {
		if !errors.Is(err, ErrNotSupported) {
			t.Fatalf("err = %v", err)
		}
	}
}

func TestApps(t *testing.T) {
	f := setup(t)

	must(t, WriteAll("hi 你好"))
	s, err := ReadAll()
	must(t, err)
	if s != "hi 你好" {
		t.Fatalf("ReadAll = %q", s)
	}
	f.take()

	must(t, TapHome())
	must(t, RunApp("com.apple.Preferences"))
	must(t, CloseApp("com.apple.Preferences"))
	id, err := ActiveApp()
	must(t, err)
	if id != "com.apple.Preferences" {
		t.Fatalf("ActiveApp = %q", id)
	}
	bundle := map[string]interface{}{"bundleId": "com.apple.Preferences"}
	expect(t, f.take(),
		req{"POST", "/wda/homescreen", nil},
		req{"POST", "/session/s1/wda/apps/launch", bundle},
		req{"POST", "/session/s1/wda/apps/terminate", bundle},
		req{"GET", "/wda/activeAppInfo", nil},
	)
}

func TestSessionAndErrors(t *testing.T) {
	f := setup(t)

	must(t, Tap(1, 1))
	f.expired = true
	must(t, Tap(1, 1))
	f.mu.Lock()
	n := 0
	for _, r := range f.reqs {
		if r.Path == "/session" {
			n++
		}
	}
	f.mu.Unlock()
	if n != 2 {
		t.Fatalf("sessions created = %d, want 2", n)
	}

	if err := call("GET", "/broken", nil, nil); err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("err = %v", err)
	}

	URL = "http://127.0.0.1:1"
	sessionID = ""
	if Tap(1, 1) == nil {
		t.Fatal("want connection error")
	}
	if w, h := GetScreenSize(); w != 0 || h != 0 {
		t.Fatalf("size = %d, %d", w, h)
	}
	if GetPixelColor(0, 0) != "" {
		t.Fatal("want empty color")
	}
}
