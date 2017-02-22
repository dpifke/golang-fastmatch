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
	"io/ioutil"
	"reflect"
	"sort"
	"testing"
)

// typeOf returns the type name of a value, including pointer dereferences.
func typeOf(v interface{}) string {
	var b bytes.Buffer
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		b.WriteRune('*')
		t = t.Elem()
	}
	b.WriteString(t.Name())
	return b.String()
}

var badFlagsTests = []struct {
	flags     []*Flag
	expect    *ErrBadFlags
	expectStr string
}{
	{
		flags: []*Flag{HasPrefix, HasSuffix},
		expect: &ErrBadFlags{
			cannotCombine: []string{"HasPrefix", "HasSuffix"},
		},
	}, {
		flags: []*Flag{Normalize, HasSuffix, Insensitive, HasPrefix},
		expect: &ErrBadFlags{
			cannotCombine: []string{"HasPrefix", "HasSuffix"},
		},
	}, {
		flags: []*Flag{Ignore('a'), IgnoreExcept('a')},
		expect: &ErrBadFlags{
			cannotCombine: []string{"Ignore", "IgnoreExcept"},
		},
	}, {
		flags: []*Flag{IgnoreExcept(Alphanumeric...), Ignore(Numbers...)},
		expect: &ErrBadFlags{
			cannotCombine: []string{"Ignore", "IgnoreExcept"},
		},
	}, {
		flags: []*Flag{StopUpon('a', 'x'), Ignore('y', 'a')},
		expect: &ErrBadFlags{
			cannotStopIgnore: []rune{'a'},
		},
	}, {
		flags: []*Flag{StopUpon('a', 'b', 'c'), Ignore('A', 'B', 'C'), Insensitive},
		expect: &ErrBadFlags{
			cannotStopIgnore: []rune{'a', 'b', 'c'},
		},
	},
}

// TestBadFlags tests that Generate returns an error if impossible flags are
// given.
func TestBadFlags(t *testing.T) {
	for _, testCase := range badFlagsTests {
		err := Generate(ioutil.Discard, map[string]string{"a": "1"}, "0", testCase.flags...)
		if err == nil {
			t.Errorf("failed to trigger ErrBadFlags")
		} else if err, ok := err.(*ErrBadFlags); !ok {
			t.Errorf("expected *ErrBadFlags, got %s: %q", typeOf(err), err.Error())
		} else {
			if errStr := err.Error(); errStr != testCase.expectStr && testCase.expectStr != "" {
				t.Errorf("expected %q, got %q", testCase.expectStr, errStr)
			}

			if testCase.expect != nil {
				sort.Strings(testCase.expect.cannotCombine)
				sort.Sort(testCase.expect.cannotStopIgnore)

				if !reflect.DeepEqual(err, testCase.expect) {
					t.Errorf("internals of returned error did not match expected")
				}
			}
		}
	}
}

var rangeTests = []struct {
	input, shouldInclude, shouldExclude []rune
}{
	{
		input:         Range('c', 'e', 'h', 'j'),
		shouldInclude: []rune{'c', 'd', 'e', 'h', 'i', 'j'},
		shouldExclude: []rune{'a', 'b', 'f', 'g', 'k', 'l'},
	}, {
		input:         Numbers,
		shouldInclude: []rune{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9'},
		shouldExclude: []rune{'a', 'A', 'z', 'Z', '!', '\n'},
	}, {
		input:         Uppercase,
		shouldInclude: []rune{'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z'},
		shouldExclude: []rune{'a', 'z', '0', '9', '!', '\n'},
	}, {
		input:         Lowercase,
		shouldInclude: []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z'},
		shouldExclude: []rune{'A', 'Z', '0', '9', '!', '\n'},
	}, {
		input:         Letters,
		shouldInclude: []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z'},
		shouldExclude: []rune{'0', '9', '!', '\n'},
	}, {
		input:         Alphanumeric,
		shouldInclude: []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9'},
		shouldExclude: []rune{'!', '\n'},
	},
}

// TestRange tests the Range function and predefined ranges.
func TestRange(t *testing.T) {
	for _, testCase := range rangeTests {
		rm := make(map[rune]bool, len(testCase.input))
		for _, r := range testCase.input {
			rm[r] = true
		}

		var missing []rune
		for _, r := range testCase.shouldInclude {
			if !rm[r] {
				missing = append(missing, r)
			}
		}
		if len(missing) > 0 {
			t.Error("runes missing from range:", quoteRunes(missing))
		}

		var extra []rune
		for _, r := range testCase.shouldExclude {
			if rm[r] {
				extra = append(extra, r)
			}
		}
		if len(extra) > 0 {
			t.Error("extra runes in range:", quoteRunes(extra))
		}
	}
}
