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
	"log"
	"strconv"

	"github.com/go-vgo/gt/cmd"
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

// TypeStr input text with string
func TypeStr(str string) error {
	in := adbIn + "text " + str
	return RunCmd(in, "input text: ")
}

// Tap tap the app
func Tap(x, y int) error {
	return Click(x, y)
}

// TapKey tap the key code
func TapKey(key string) error {
	in := adbIn + "keyevent "
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

// Click tap the app
func Click(x, y int) error {
	xy := strconv.Itoa(x) + " " + strconv.Itoa(y)
	in := adbIn + "tap " + xy

	return RunCmd(in, "tap app: ")
}

// Scroll scroll the focus x, y to endX, endY
func Scroll(x, y, endX, endY int) error {
	str := strconv.Itoa(x) + " " + strconv.Itoa(y) + " " +
		strconv.Itoa(endX) + " " + strconv.Itoa(endY)
	in := adbIn + "swipe " + str

	return RunCmd(in, "scroll: ")
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

// SaveCapture save the capture to savePath (PC)
func SaveCapture(path, savePath string) error {
	in := adbs + "pull /sdcard/" + path + " " + savePath
	return RunCmd(in, "save capture: ")
}
