// run

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test floating-point comparison involving NaN.

package main

import "math"

type floatTest struct {
	name string
	expr bool
	want bool
}

var nan float64 = math.NaN()
var f float64 = 1

var tests = []floatTest{
	{"nan == nan", nan == nan, false},
	{"nan != nan", nan != nan, true},
	{"nan < nan", nan < nan, false},
	{"nan > nan", nan > nan, false},
	{"nan <= nan", nan <= nan, false},
	{"nan >= nan", nan >= nan, false},
	{"f == nan", f == nan, false},
	{"f != nan", f != nan, true},
	{"f < nan", f < nan, false},
	{"f > nan", f > nan, false},
	{"f <= nan", f <= nan, false},
	{"f >= nan", f >= nan, false},
	{"nan == f", nan == f, false},
	{"nan != f", nan != f, true},
	{"nan < f", nan < f, false},
	{"nan > f", nan > f, false},
	{"nan <= f", nan <= f, false},
	{"nan >= f", nan >= f, false},
	{"!(nan == nan)", !(nan == nan), true},
	{"!(nan != nan)", !(nan != nan), false},
	{"!(nan < nan)", !(nan < nan), true},
	{"!(nan > nan)", !(nan > nan), true},
	{"!(nan <= nan)", !(nan <= nan), true},
	{"!(nan >= nan)", !(nan >= nan), true},
	{"!(f == nan)", !(f == nan), true},
	{"!(f != nan)", !(f != nan), false},
	{"!(f < nan)", !(f < nan), true},
	{"!(f > nan)", !(f > nan), true},
	{"!(f <= nan)", !(f <= nan), true},
	{"!(f >= nan)", !(f >= nan), true},
	{"!(nan == f)", !(nan == f), true},
	{"!(nan != f)", !(nan != f), false},
	{"!(nan < f)", !(nan < f), true},
	{"!(nan > f)", !(nan > f), true},
	{"!(nan <= f)", !(nan <= f), true},
	{"!(nan >= f)", !(nan >= f), true},
	{"!!(nan == nan)", !!(nan == nan), false},
	{"!!(nan != nan)", !!(nan != nan), true},
	{"!!(nan < nan)", !!(nan < nan), false},
	{"!!(nan > nan)", !!(nan > nan), false},
	{"!!(nan <= nan)", !!(nan <= nan), false},
	{"!!(nan >= nan)", !!(nan >= nan), false},
	{"!!(f == nan)", !!(f == nan), false},
	{"!!(f != nan)", !!(f != nan), true},
	{"!!(f < nan)", !!(f < nan), false},
	{"!!(f > nan)", !!(f > nan), false},
	{"!!(f <= nan)", !!(f <= nan), false},
	{"!!(f >= nan)", !!(f >= nan), false},
	{"!!(nan == f)", !!(nan == f), false},
	{"!!(nan != f)", !!(nan != f), true},
	{"!!(nan < f)", !!(nan < f), false},
	{"!!(nan > f)", !!(nan > f), false},
	{"!!(nan <= f)", !!(nan <= f), false},
	{"!!(nan >= f)", !!(nan >= f), false},
}

func main() {
	bad := false
	for _, t := range tests {
		if t.expr != t.want {
			if !bad {
				bad = true
				println("BUG: floatcmp")
			}
			println(t.name, "=", t.expr, "want", t.want)
		}
	}
	if bad {
		panic("floatcmp failed")
	}
}
