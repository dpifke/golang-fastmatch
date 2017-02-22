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
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var sortTestCases = []struct {
	input, sorted sliceOfStringSlices
}{
	{
		input:  [][]string{[]string{"b"}, []string{"a"}, []string{"c"}},
		sorted: [][]string{[]string{"a"}, []string{"b"}, []string{"c"}},
	}, {
		input:  [][]string{[]string{"b", "a"}, []string{"a", "b"}},
		sorted: [][]string{[]string{"a", "b"}, []string{"b", "a"}},
	}, {
		input:  [][]string{[]string{"b", "a"}, nil},
		sorted: [][]string{nil, []string{"b", "a"}},
	}, {
		input:  [][]string{nil, []string{"b"}, nil},
		sorted: [][]string{nil, nil, []string{"b"}},
	},
}

// TestSort tests sorting a slice of string slices.
func TestSort(t *testing.T) {
	for _, testCase := range sortTestCases {
		sort.Sort(testCase.input)
		if !reflect.DeepEqual(testCase.input, testCase.sorted) {
			t.Errorf("got %q, expected %q", testCase.input, testCase.sorted)
		}
	}
}

// TestErrAmbiguous tests construction and stringification of ErrAmbiguous.
func TestErrAmbiguous(t *testing.T) {
	e := new(ErrAmbiguous)
	e.add(nil, "foo", "bar")
	e.add(nil, "foo", "baz")
	e.add(nil, "hello", "world")

	expect := []map[string]bool{
		map[string]bool{"foo": true, "bar": true, "baz": true},
		map[string]bool{"hello": true, "world": true},
	}
	if !reflect.DeepEqual(e.keys, expect) {
		t.Fail()
	}

	for _, key := range []string{"foo", "bar", "baz", "hello", "world"} {
		if strings.Count(e.Error(), strconv.Quote(key)) != 1 {
			t.Errorf("expected exactly 1 instance of %q in error message %q", key, e.Error())
		}
	}
}

var ambiguityTestCases = []struct {
	descr     string
	flags     []*Flag
	cases     map[string]string
	ambiguous sliceOfStringSlices
	maxState  uint64
}{
	{
		descr: "Inensitive",
		flags: []*Flag{Insensitive},
		cases: map[string]string{
			"Foo": "1", "foo": "2",
			"Bar": "3", "bar": "4",
			"bat": "5",
		},
		ambiguous: sliceOfStringSlices{
			[]string{"Foo", "foo"},
			[]string{"Bar", "bar"},
		},
	}, {
		descr: "Inensitive (with HasPrefix)",
		flags: []*Flag{Insensitive, HasPrefix},
		cases: map[string]string{
			"Foo": "1", "foo": "2",
			"Bar": "3", "bar": "4",
			"bat": "5",
		},
		ambiguous: sliceOfStringSlices{
			[]string{"Foo", "foo"},
			[]string{"Bar", "bar"},
		},
	}, {
		descr: "Inensitive (with HasSuffix)",
		flags: []*Flag{Insensitive, HasSuffix},
		cases: map[string]string{
			"Foo": "1", "foo": "2",
			"Bar": "3", "bar": "4",
			"bat": "5",
		},
		ambiguous: sliceOfStringSlices{
			[]string{"Foo", "foo"},
			[]string{"Bar", "bar"},
		},
	}, {
		descr: "Inensitive (chained state machine)",
		flags: []*Flag{Insensitive},
		cases: map[string]string{
			"abcdefghijklmnop": "1", "ABCdefghijklmnop": "2",
			"ponmlkjihgfedcba": "3", "ponmlkjihgfedCBA": "4",
			"zyxwvutsrqponmlk": "5",
		},
		ambiguous: sliceOfStringSlices{
			[]string{"abcdefghijklmnop", "ABCdefghijklmnop"},
			[]string{"ponmlkjihgfedcba", "ponmlkjihgfedCBA"},
		},
		maxState: 0xff,
	}, {
		descr: "HasPrefix",
		flags: []*Flag{HasPrefix},
		cases: map[string]string{
			"foo": "1", "f": "2",
			"bar": "3", "b": "4",
			"qoo": "5",
		},
		ambiguous: sliceOfStringSlices{
			[]string{"foo", "f"},
			[]string{"bar", "b"},
		},
	}, {
		descr: "HasPrefix",
		flags: []*Flag{HasPrefix},
		cases: map[string]string{
			"foo": "1", "fo": "2",
			"bar": "3", "ba": "4",
			"far": "5", "fa": "6",
			"tar": "5",
		},
		ambiguous: sliceOfStringSlices{
			[]string{"foo", "fo"},
			[]string{"bar", "ba"},
			[]string{"far", "fa"},
		},
	}, {
		descr: "HasPrefix (chained state machine)",
		flags: []*Flag{HasPrefix},
		cases: map[string]string{
			"abcdefghijklmnop": "1", "abcdefghijklm": "2",
			"ponmlkjihgfedcba": "3", "po": "4",
			"zyxwvutsrqponmlk": "5",
		},
		ambiguous: sliceOfStringSlices{
			[]string{"abcdefghijklmnop", "abcdefghijklm"},
			[]string{"ponmlkjihgfedcba", "po"},
		},
		maxState: 0xff,
	}, {
		descr: "HasSuffix (different final rune)",
		flags: []*Flag{HasSuffix},
		cases: map[string]string{
			"oof": "1", "f": "2",
			"bar": "3",
		},
		ambiguous: sliceOfStringSlices{
			[]string{"oof", "f"},
		},
	}, {
		descr: "HasSuffix (different intermediate state)",
		flags: []*Flag{HasSuffix},
		cases: map[string]string{
			"oof": "1", "of": "2",
		},
		ambiguous: sliceOfStringSlices{
			[]string{"oof", "of"},
		},
	}, {
		descr: "StopUpon",
		flags: []*Flag{StopUpon('.')},
		cases: map[string]string{
			"foo": "1", "foo.": "2",
			"bar.x": "3", "bar.y": "4",
			"far": "5", "quick": "6",
		},
		ambiguous: sliceOfStringSlices{
			[]string{"foo", "foo."},
			[]string{"bar.x", "bar.y"},
		},
	}, {
		descr: "Ignore",
		flags: []*Flag{Ignore('.')},
		cases: map[string]string{
			"foo": "1", "foo.": "2",
			"barx": "3", "bar.x": "4",
			"far": "5", "quick": "6",
		},
		ambiguous: sliceOfStringSlices{
			[]string{"foo", "foo."},
			[]string{"barx", "bar.x"},
		},
	}, {
		descr: "IgnoreExcept",
		flags: []*Flag{IgnoreExcept('0', '1')},
		cases: map[string]string{
			"f0o0o": "1", "00": "2",
			"ba11r": "3", "11": "4",
			"101": "5", "010": "6",
		},
		ambiguous: sliceOfStringSlices{
			[]string{"f0o0o", "00"},
			[]string{"ba11r", "11"},
		},
	},
}

// TestAmbiguity tests that an error is returned if we're asked to generate
// code with ambiguous matches.
func TestAmbiguity(t *testing.T) {
	for _, testCase := range ambiguityTestCases {
		testCase.ambiguous.sort()

		oldMaxState := maxState
		if testCase.maxState != 0 {
			maxState = testCase.maxState
		}

		if err := Generate(ioutil.Discard, testCase.cases, "0", testCase.flags...); err == nil {
			t.Errorf("failed to detect %s ambiguity", testCase.descr)
		} else if err, ok := err.(*ErrAmbiguous); !ok {
			t.Errorf("expected *ErrAmbiguous, got %s: %q", typeOf(err), err.Error())
		} else if !reflect.DeepEqual(err.sortedKeys(), [][]string(testCase.ambiguous)) {
			t.Errorf("incorrect ambiguous key list for %s: got %s, expected %s", testCase.descr, err.sortedKeys(), testCase.ambiguous)
		}

		// Remove the ambiguity by making all return values the same:
		nonAmbiguous := make(map[string]string, len(testCase.cases))
		for key := range testCase.cases {
			nonAmbiguous[key] = "1"
		}
		if err := Generate(ioutil.Discard, nonAmbiguous, "0", testCase.flags...); err != nil {
			t.Errorf("error from non-ambiguous %s cases: %s", testCase.descr, err.Error())
		}

		if testCase.maxState != 0 {
			maxState = oldMaxState
		}
	}
}

// TestReverseAmbiguity tests that an error is returned if GenerateReverse is
// called with multiple strings mapping to the same expression.
func TestReverseAmbiguity(t *testing.T) {
	expect := sliceOfStringSlices{
		[]string{"foo", "bar"},
		[]string{"baz", "bat"},
	}
	expect.sort()

	err := GenerateReverse(ioutil.Discard, map[string]string{
		"foo": "1",
		"bar": "1",
		"baz": "2",
		"bat": "2",
	}, `""`)
	if err == nil {
		t.Errorf("failed to detect ambiguity")
	} else if err, ok := err.(*ErrAmbiguous); !ok {
		t.Errorf("expected *ErrAmbiguous, got %s: %q", typeOf(err), err.Error())
	} else if !reflect.DeepEqual(err.sortedKeys(), [][]string(expect)) {
		t.Errorf("incorrect ambiguous key list")
	}
}
