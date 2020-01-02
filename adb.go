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
	adb   = "adb shell "
	adbIn = adb + "input "
)

// RunApp run the android app
func RunApp(appPath string) error {
	in := adb + "start am start -n " + appPath
	out, e, err := cmd.Run(in)
	log.Println("run app: ", out, e, err)

	return err
}

// CloseApp close the android app
func CloseApp(pkgName string) error {
	in := adb + "am force-stop " + pkgName
	out, e, err := cmd.Run(in)
	log.Println("close app: ", out, e, err)

	return err
}

// TypeStr input text with string
func TypeStr(str string) error {
	in := adbIn + "text " + str
	out, e, err := cmd.Run(in)
	log.Println("input text: ", out, e, err)

	return err
}

// Tap tap the app
func Tap(x, y int) error {
	return Click(x, y)
}

// Click tap the app
func Click(x, y int) error {
	xy := strconv.Itoa(x) + " " + strconv.Itoa(y)
	in := adbIn + "tap " + xy

	out, e, err := cmd.Run(in)
	log.Println("tap app: ", out, e, err)

	return err
}
