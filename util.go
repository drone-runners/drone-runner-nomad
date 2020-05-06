// Copyright 2020 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

package main

import (
	"time"

	"github.com/dchest/uniuri"
)

// random generator function
var random = func() string {
	return "drone-job-" + uniuri.NewLen(12)
}

// stringToPtr returns the pointer to a string
func stringToPtr(str string) *string {
	return &str
}

// intToPtr returns the pointer to a int
func intToPtr(i int) *int {
	return &i
}

// durationToPtr returns the pointer to a duration
func durationToPtr(dur time.Duration) *time.Duration {
	return &dur
}
