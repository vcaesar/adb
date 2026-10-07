# adb

[robotgo](https://github.com/go-vgo/robotgo) compatible API for Android (adb) and iOS (WebDriverAgent).

A touch screen has no cursor, so `Move` sets a virtual pointer, `Click` taps at it,
`Click("right")` long presses, `Toggle`/`MouseDown`/`MouseUp` tap, long press or swipe,
`Scroll` swipes from the screen center, and `CaptureImg`/`SaveCapture`/`GetPixelColor`
use device screenshots.

## Android

Needs `adb` in `PATH`; set `adb.Serial` when several devices are connected.

```Go
package main

import "github.com/vcaesar/adb"

func main() {
	adb.Move(500, 1200)
	adb.Click()
	adb.Click(100, 200)
	adb.ScrollDir(5, "down")
	adb.KeyTap("enter")
	adb.TypeStr("hello")
	adb.SaveCapture("screen.png")
}
```

## iOS

Start [WebDriverAgent](https://github.com/appium/WebDriverAgent) and forward its port,
e.g. with [go-ios](https://github.com/danielpaulus/go-ios): `ios runwda` and `ios forward 8100 8100`.
Set `ios.URL` for another address. Coordinates are points.

```Go
package main

import "github.com/vcaesar/adb/ios"

func main() {
	ios.RunApp("com.apple.Preferences")
	ios.MoveClick(200, 400)
	ios.ScrollDir(5, "down")
	ios.TypeStr("hello")
	ios.KeyTap("home")
	ios.SaveCapture("screen.png")
}
```

## Not supported

- `KeyToggle`, `KeyDown` and `KeyUp` return `ErrNotSupported`.
- Android `TypeStr` takes ASCII only, and modifier keys need Android 13 or later.
- On iOS, special keys and modifiers need iPadOS 17+ (Xcode 15).
