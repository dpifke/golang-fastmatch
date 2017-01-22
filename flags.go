// Copyright (c) 2014-2016 Dave Pifke.
//
// Redistribution and use in source and binary forms, with or without
// modification, is permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice,
//    this list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.
//
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package fastmatch

import (
	"bytes"
	"sort"
	"strconv"
)

// ErrBadFlags is returned when nonsensical flags are passed to Generate.
type ErrBadFlags struct {
	cannotCombine []string
}

func (e *ErrBadFlags) Error() string {
	var b bytes.Buffer

	sort.Strings(e.cannotCombine)
	for n, key := range e.cannotCombine {
		if n == 0 {
			b.WriteString("flags are mutually exclusive: ")
		} else if n == len(e.cannotCombine)-1 {
			if n == 1 {
				// last of two-item list
				b.WriteString(" and ")
			} else {
				// last of longer list, with Oxford comma
				// http://i3.kym-cdn.com/photos/images/newsfeed/000/946/427/5a4.jpg
				b.WriteString(", and ")
			}
		} else {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(key))
	}

	return b.String()
}

// Flag can be passed to Generate and GenerateReverse to modify the functions'
// behavior.  Users of this package should not instantiate their own Flags.
// Rather, they should use one of HasPrefix, HasSuffix, Insensitive,
// Normalize, or the return value from Equivalent().  Unknown Flags are
// ignored.
type Flag struct {
	equivalent []rune
}

// Insensitive is a flag, which can be passed to Generate, to specify that
// matching should be case-insensitive.
var Insensitive = new(Flag)

// Normalize is a flag, which can be passed to Generate, to specify that
// matching should be done without regard to diacritics, accents, etc.
//
// This is currently just a placeholder, and has no effect yet on the
// generated code.
var Normalize = new(Flag)

// Equivalent is a flag, which can be passed to Generate, to specify
// runes that should be treated identically when matching.
func Equivalent(runes ...rune) *Flag {
	return &Flag{equivalent: runes}
}

// HasPrefix is a flag, which can be passed to Generate, to specify that
// runes proceeding a match should be ignored.
//
// Matching stops as soon as a match is found, thus "foo" and "f" are
// ambiguous cases when HasPrefix is specified.  Generate returns an error if
// ambiguity is detected.
var HasPrefix = new(Flag)

// HasSuffix is a flag, which can be passed to Generate, to match the end of
// the input string, in the same manner HasPrefix performs a match of the
// beginning of the string.
var HasSuffix = new(Flag)
