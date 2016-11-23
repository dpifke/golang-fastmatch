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
	"io/ioutil"
	"strconv"
	"strings"
	"testing"
)

var ambiguityTestCases = []struct {
	descr string
	cases map[string]string
	flags []*Flag
}{
	{
		descr: "Inensitive",
		cases: map[string]string{
			"Foo": "1", "foo": "2",
			"Bar": "3", "bar": "4",
		},
		flags: []*Flag{Insensitive},
	}, {
		descr: "Inensitive (with HasPrefix)",
		cases: map[string]string{
			"Foo": "1", "foo": "2",
			"Bar": "3", "bar": "4",
		},
		flags: []*Flag{Insensitive, HasPrefix},
	}, {
		descr: "Inensitive (with HaSuffix)",
		cases: map[string]string{
			"Foo": "1", "foo": "2",
			"Bar": "3", "bar": "4",
		},
		flags: []*Flag{Insensitive, HasSuffix},
	}, {
		descr: "HasPrefix",
		cases: map[string]string{
			"foo": "1", "f": "2",
		},
		flags: []*Flag{HasPrefix},
	}, {
		descr: "HasPrefix",
		cases: map[string]string{
			"foo": "1", "fo": "2",
			"bar": "3", "ba": "4",
			"far": "5", "fa": "6",
		},
		flags: []*Flag{HasPrefix},
	}, {
		descr: "HasPrefix (different final rune)",
		cases: map[string]string{
			"oof": "1", "f": "2",
		},
		flags: []*Flag{HasSuffix},
	}, {
		descr: "HasPrefix (different intermediate state)",
		cases: map[string]string{
			"oof": "1", "of": "2",
		},
		flags: []*Flag{HasSuffix},
	},
}

// TestAmbiguity tests that an error is returned if we're asked to generate
// code with ambiguous matches.
func TestAmbiguity(t *testing.T) {
	for _, testCase := range ambiguityTestCases {
		if err := Generate(ioutil.Discard, testCase.cases, "0", testCase.flags...); err == nil {
			t.Errorf("failed to detect %s ambiguity", testCase.descr)
		} else {
			for key := range testCase.cases {
				if strings.Count(err.Error(), strconv.Quote(key)) != 1 {
					t.Errorf("expected exactly 1 instance of %q in error message %q", key, err.Error())
				}
			}
		}

		// Remove the ambiguity by making all return values the same:
		nonAmbiguous := make(map[string]string, len(testCase.cases))
		for key := range testCase.cases {
			nonAmbiguous[key] = "1"
		}
		if err := Generate(ioutil.Discard, nonAmbiguous, "0", testCase.flags...); err != nil {
			t.Errorf("error from non-ambiguous %s cases: %s", testCase.descr, err.Error())
		}
	}
}

// TestReverseAmbiguity tests that an error is returned if GenerateReverse is
// called with multiple strings mapping to the same expression.
func TestReverseAmbiguity(t *testing.T) {
	err := GenerateReverse(ioutil.Discard, map[string]string{
		"foo": "1",
		"bar": "1",
		"baz": "2",
		"bat": "2",
	}, `""`)
	if err == nil {
		t.Errorf("failed to detect ambiguity")
	}
}
