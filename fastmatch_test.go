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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// TestRuneEquivalents tests the construction of runeEquivalents via flags
// (including sorting, de-duping, and transitivity), as well as evaluation of
// equivalence of individual runes.
func TestRuneEquivalents(t *testing.T) {
	equiv := makeRuneEquivalents(
		Equivalent('a', 'b'),
		Equivalent('B', 'c'),
		Insensitive,
		Equivalent('c', 'd'),
		Equivalent('a', 'c', 'd'),
	)

	expect := []rune{'A', 'B', 'C', 'D', 'a', 'b', 'c', 'd'}
	for _, r := range expect {
		if !reflect.DeepEqual(expect, equiv.lookup(r)) {
			t.Errorf("expected %q, got %q looking up %q", expect, equiv.lookup(r), r)
		}
	}

	expect = []rune{'E', 'e'}
	if !reflect.DeepEqual(expect, equiv.lookup('e')) {
		t.Errorf("expected %q, got %q looking up %q", expect, equiv.lookup('e'), 'e')
	} else {
		expectStr := `'E', 'e'`
		if equiv.lookupString('e') != expectStr {
			t.Errorf("expected %q, got %q", expectStr, equiv.lookup('e'))
		}

		if !equiv.isEquiv('E', 'e') {
			t.Error("should have been equivalent, but wasn't:", expectStr)
		}
	}

	expect = []rune{'.'}
	if !reflect.DeepEqual(expect, equiv.lookup('.')) {
		t.Errorf("expected %q, got %q looking up %q", expect, equiv.lookup('.'), '.')
	} else if equiv.isEquiv('.', 'e') {
		t.Error("should not have been equivalent, but was: '.', 'e'")
	}
}

// TestUniqueRunes tests deconstructing a series of strings at a given offset
// to determine the unique runes, taking equivalence into account.
func TestUniqueRunes(t *testing.T) {
	keys := []string{"abc123", "ABC123", "DEF78"}
	expect := [][]rune{
		[]rune{'D', 'a'}, // note sort order (capitals < lowercase)
		[]rune{'E', 'b'},
		[]rune{'F', 'c'},
		[]rune{'1', '7'},
		[]rune{'2', '8'},
		[]rune{'3'},
	}
	equiv := makeRuneEquivalents(Insensitive)

	for n := range expect {
		result := equiv.uniqueAtOffset(keys, n)
		if !reflect.DeepEqual(expect[n], result) {
			t.Errorf("expected %q, got %q at offset %d", expect[n], result, n)
		}
	}
}

// generateFunc matches both Generate() and GenerateReverse().
type generateFunc func(io.Writer, map[string]string, string, ...*Flag) error

// generateRunnable creates a temporary directory, adds it GOPATH, and uses
// Generate to create runnable string matcher code therein.
//
// A cleanup function is returned, which should be executed by the caller at
// the completion of the test.  This removes the temporary directory and
// restores GOPATH and the current working directory.
func generateRunnable(fn generateFunc, retType string, cases map[string]string, none string, flags ...*Flag) (func(), error) {
	cleanup := func() {}

	var out io.Writer
	dir, err := ioutil.TempDir("", "fastmatch_test")
	if err == nil {
		savedWd, _ := os.Getwd()
		err = os.Chdir(dir)
		if err == nil {
			savedGopath := os.Getenv("GOPATH")
			os.Setenv("GOPATH", fmt.Sprintf("%s:%s", dir, savedGopath))
			out, err = os.Create("test.go")
			cleanup = func() {
				os.Setenv("GOPATH", savedGopath)
				os.Chdir(savedWd)
				os.RemoveAll(dir)
			}
		}
	}
	if err != nil {
		return cleanup, err
	}

	fmt.Fprintln(out, "package main")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "import (")
	fmt.Fprintln(out, "\t\"fmt\"")
	fmt.Fprintln(out, "\t\"os\"")
	fmt.Fprintln(out, ")")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "func match(input string)", retType, "{")
	err = fn(out, cases, none, flags...)
	if err != nil {
		return cleanup, err
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "func main() {")
	fmt.Fprintln(out, "\tfmt.Println(match(os.Args[1]))")
	fmt.Fprintln(out, "}")

	return cleanup, nil
}

// expectMatch uses `go run` to execute our generated test.go file.  It passes
// the provided input and compares the output to what the test expects.
func expectMatch(t *testing.T, input, expect string) {
	cmd := exec.Command("go", "run", "test.go", input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %s", err.Error(), strings.TrimSpace(string(out)))
	}

	outs := strings.TrimSpace(string(out))
	if outs != expect {
		t.Errorf("expected %q, got %q for input %q", expect, outs, input)
	}
}

// TestNoFlags tests a simple matcher.
func TestNoFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compiled tests in short mode")
	}

	cleanup, err := generateRunnable(Generate, "int", map[string]string{
		"foo": "1",
		"bar": "2",
		"baz": "3",
	}, "0")
	defer cleanup()
	if err != nil {
		t.Fatalf(err.Error())
	}

	expectMatch(t, "foo", "1")
	expectMatch(t, "bar", "2")
	expectMatch(t, "baz", "3")
	expectMatch(t, "bat", "0")
	expectMatch(t, "bazz", "0")
}

// TestInsensitive tests a case-insensitive matcher.
func TestInsensitive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compiled tests in short mode")
	}

	cleanup, err := generateRunnable(Generate, "int", map[string]string{
		"foo": "1",
		"Bar": "2",
		"baz": "3",
	}, "0", Insensitive)
	defer cleanup()
	if err != nil {
		t.Fatalf(err.Error())
	}

	expectMatch(t, "Foo", "1")
	expectMatch(t, "BAR", "2")
	expectMatch(t, "baz", "3")
	expectMatch(t, "bat", "0")
}

// TestEquivalent tests a matcher which makes use of the Equivalent flag.
func TestEquivalent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compiled tests in short mode")
	}

	cleanup, err := generateRunnable(Generate, "int", map[string]string{
		"foo00000": "1",
		"bar11111": "2",
	}, "0", Equivalent('0', '1', '2', '3', '4', '5', '6', '7', '8', '9'))
	defer cleanup()
	if err != nil {
		t.Fatalf(err.Error())
	}

	expectMatch(t, "foo90210", "1")
	expectMatch(t, "foo11111", "1")
	expectMatch(t, "bar00000", "2")
	expectMatch(t, "bar12345", "2")
	expectMatch(t, "fooabcde", "0")
	expectMatch(t, "barzyxwv", "0")
}

// TestHasPrefix tests a prefix matcher.
func TestHasPrefix(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compiled tests in short mode")
	}

	cleanup, err := generateRunnable(Generate, "int", map[string]string{
		"f":   "1",
		"Bar": "2",
		"baz": "3",
	}, "0", HasPrefix, Insensitive)
	defer cleanup()
	if err != nil {
		t.Fatalf(err.Error())
	}

	expectMatch(t, "f", "1")
	expectMatch(t, "foo", "1")
	expectMatch(t, "FOO", "1")
	expectMatch(t, "bar", "2")
	expectMatch(t, "bart", "2")
	expectMatch(t, "bz", "0")
	expectMatch(t, "bzz", "0")
}

// TestHasSuffix tests a suffix matcher.
func TestHasSuffix(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compiled tests in short mode")
	}

	cleanup, err := generateRunnable(Generate, "int", map[string]string{
		"o":  "1",
		"ar": "2",
	}, "0", HasSuffix, Insensitive)
	defer cleanup()
	if err != nil {
		t.Fatalf(err.Error())
	}

	expectMatch(t, "o", "1")
	expectMatch(t, "flo", "1")
	expectMatch(t, "FLO", "1")
	expectMatch(t, "bao", "1")
	expectMatch(t, "bar", "2")
	expectMatch(t, "baz", "0")
}

// TestReverse tests a simple reverse matcher.
func TestReverse(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compiled tests in short mode")
	}

	cleanup, err := generateRunnable(GenerateReverse, "string", map[string]string{
		"foo": `"1"`,
		"bar": `"2"`,
	}, `"baz"`)
	defer cleanup()
	if err != nil {
		t.Fatalf(err.Error())
	}

	expectMatch(t, "1", "foo")
	expectMatch(t, "2", "bar")
	expectMatch(t, "0", "baz")
}

// TestOverflow attempts to Generate matcher code for strings that are too
// long, which overflow the uint64 state machine.
func TestOverflow(t *testing.T) {
	tooLong1 := "Anything longer than about 64 characters should do nicely.  But"
	tooLong2 := "we need more than one match, so that the state counter is used."
	err := Generate(ioutil.Discard, map[string]string{
		tooLong1: "1",
		tooLong2: "2",
	}, "0")
	if err != ErrOverflow {
		t.Fatalf("long match didn't trigger ErrOverflow")
	}

	err = Generate(ioutil.Discard, map[string]string{
		tooLong1: "1",
	}, "0")
	if err != nil {
		t.Errorf("a single match shouldn't overflow, since state machine is unnecessary")
	}
}

// TestBadFlags tests that Generate complains if passed impossible flags.
func TestBadFlags(t *testing.T) {
	for _, flags := range [][]*Flag{
		[]*Flag{HasPrefix, HasSuffix},
		[]*Flag{Normalize, HasSuffix, Insensitive, HasPrefix},
	} {
		if err := Generate(ioutil.Discard, map[string]string{"a": "1"}, "0", flags...); err != ErrBadFlags {
			t.Errorf("failed to trigger ErrBadFlags")
		}
	}
}

// TestBadWriter tests that Generate and GenerateReverse return an error
// if passed an unusable io.Writer.
func TestBadWriter(t *testing.T) {
	f, _ := ioutil.TempFile("", "fastmatch_test")
	f.Close()
	os.Remove(f.Name())

	if err := Generate(f, map[string]string{"a": "1"}, "0"); err == nil {
		t.Errorf("no error from Generate on closed io.Writer")
	}
	if err := Generate(f, map[string]string{"a": "1"}, "0", HasPrefix); err == nil {
		t.Errorf("no error from Generate (with HasPrefix) on closed io.Writer")
	}
	if err := GenerateReverse(f, map[string]string{"a": "1"}, `""`); err == nil {
		t.Errorf("no error from GenerateReverse on closed io.Writer")
	}
}

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
