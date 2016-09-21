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
	"strings"
	"testing"
)

// TestRuneEquivalents tests the construction of runeEquivalents via flags,
// including sorting, de-duping, and transitivity.
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
	}

	expect = []rune{'.'}
	if !reflect.DeepEqual(expect, equiv.lookup('.')) {
		t.Errorf("expected %q, got %q looking up %q", expect, equiv.lookup('.'), '.')
	}
}

// generateFunc matches both Generate() and GenerateReverse().
type generateFunc func(io.Writer, map[string]string, string, ...*flag) error

// generateRunnable creates a temporary directory, adds it GOPATH, and uses
// Generate to create runnable string matcher code therein.
//
// A cleanup function is returned, which should be executed by the caller at
// the completion of the test.  This removes the temporary directory and
// restores GOPATH and the current working directory.
func generateRunnable(fn generateFunc, retType string, cases map[string]string, none string, flags ...*flag) (error, func()) {
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
		return err, cleanup
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
		return err, cleanup
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "func main() {")
	fmt.Fprintln(out, "\tfmt.Println(match(os.Args[1]))")
	fmt.Fprintln(out, "}")

	return nil, cleanup
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
	err, cleanup := generateRunnable(Generate, "int", map[string]string{
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
}

// TestInsensitive tests a case-insensitive matcher.
func TestInsensitive(t *testing.T) {
	err, cleanup := generateRunnable(Generate, "int", map[string]string{
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
	err, cleanup := generateRunnable(Generate, "int", map[string]string{
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

// TestReverse tests a simple reverse matcher.
func TestReverse(t *testing.T) {
	err, cleanup := generateRunnable(GenerateReverse, "string", map[string]string{
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

// TestOverflow attempts to Generate matcher code for a string that is too
// long, and overflows the uint64 state machine.
func TestOverflow(t *testing.T) {
	tooLong := "Anything longer than about 64 characters should do.  This should do nicely."
	err := Generate(ioutil.Discard, map[string]string{tooLong: "1"}, "0")
	if err == nil {
		t.Errorf("long match didn't trigger overflow")
	}
}

// TestBadWriter tests that Generate and GenerateReverse return an error
// if passed an unusable io.Writer.
func TestBadFileHandle(t *testing.T) {
	f, _ := ioutil.TempFile("", "fastmatch_test")
	f.Close()
	os.Remove(f.Name())

	if err := Generate(f, map[string]string{}, "0"); err == nil {
		t.Errorf("no error from Generate on closed io.Writer")
	}
	if err := GenerateReverse(f, map[string]string{}, `""`); err == nil {
		t.Errorf("no error from GenerateReverse on closed io.Writer")
	}
}

// TestAmbiguity tests that an error is returned if we're asked to generate
// code with ambiguous matches, i.e. two strings that are equivalent to each
// other but should return different values.
func TestAmbiguity(t *testing.T) {
	err := Generate(ioutil.Discard, map[string]string{
		"Foo": "1",
		"foo": "2",
		"Bar": "3",
		"bar": "4",
	}, "0", Insensitive)
	if err == nil {
		t.Errorf("failed to detect ambiguity")
	}

	// At first glance, this looks ambiguous, but since the equivalent
	// strings both return the same value, it's actually OK.
	err = Generate(ioutil.Discard, map[string]string{
		"Foo": "1",
		"foo": "1",
	}, "0", Insensitive)
	if err != nil {
		t.Errorf("erroneously detected \"Foo\" = 1 and \"foo\" = 1 as ambiguous: %s", err.Error())
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
