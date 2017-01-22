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
	"strings"
	"testing"
)

// testDirection is passed to generateRunnable to specify whether we should
// use Generate or GenerateReverse for a particular test.
type testDirection bool

const (
	match        testDirection = true  // use Generate
	reverseMatch testDirection = false // use GenerateReverse
)

// generateRunnable creates a temporary directory, adds it GOPATH, and uses
// Generate or GenerateReverse to create runnable string matcher code therein.
//
// GenerateTest is also run, to generate and run the automated self-test;
// failures are reported, however are non-fatal.
//
// A cleanup function is returned, which should be executed by the caller at
// the completion of the test.  This removes the temporary directory and
// restores GOPATH and the current working directory.
func generateRunnable(t *testing.T, which testDirection, retType string, cases map[string]string, none string, flags ...*Flag) (func(), error) {
	cleanup := func() {}

	var out, testOut io.Writer
	dir, err := ioutil.TempDir("", "fastmatch_test")
	if err == nil {
		savedWd, _ := os.Getwd()
		err = os.Chdir(dir)
		if err == nil {
			savedGopath := os.Getenv("GOPATH")
			os.Setenv("GOPATH", fmt.Sprintf("%s:%s", dir, savedGopath))
			cleanup = func() {
				os.Setenv("GOPATH", savedGopath)
				os.Chdir(savedWd)
				os.RemoveAll(dir)
			}
			out, err = os.Create("generated.go")
			if err == nil {
				testOut, err = os.Create("generated_test.go")
			}
		}
	}
	if err != nil {
		return cleanup, err
	}

	_, err = fmt.Fprintln(out, "package main")
	if err != nil {
		return cleanup, err
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "import (")
	fmt.Fprintln(out, "\t\"fmt\"")
	fmt.Fprintln(out, "\t\"os\"")
	fmt.Fprintln(out, ")")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "func match(input string)", retType, "{")
	if which == match {
		err = Generate(out, cases, none, flags...)
	} else {
		err = GenerateReverse(out, cases, none, flags...)
	}
	if err != nil {
		return cleanup, err
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "func main() {")
	fmt.Fprintln(out, "\tfmt.Println(match(os.Args[1]))")
	_, err = fmt.Fprintln(out, "}")

	// Also generate and run the self-test.  Errors generating or running
	// the automated tests are recorded, but are not fatal.
	var fwd, rev string
	if which == match {
		fwd = "match(%q)"
		rev = ""
	} else {
		fwd = ""
		rev = "match(%s)"
	}
	_, testErr := fmt.Fprintln(testOut, "package main")
	if testErr == nil {
		fmt.Fprintln(testOut)
		fmt.Fprintln(testOut, "import \"testing\"")
		fmt.Fprintln(testOut)
		fmt.Fprintln(testOut, "func TestMatch(t *testing.T) {")
		testErr = GenerateTest(testOut, fwd, rev, cases, flags...)
	}
	if testErr == nil {
		testResult, err := exec.Command("go", "test").CombinedOutput()
		if err != nil {
			t.Errorf("%s: %s", err.Error(), strings.TrimSpace(string(testResult)))
		}
	} else {
		t.Errorf("failed to generate test: %s", testErr.Error())
	}

	return cleanup, err
}

// expectMatch uses `go run` to execute our generated test.go file.  It passes
// the provided input and compares the output to what the test expects.
func expectMatch(t *testing.T, input, expect string) {
	cmd := exec.Command("go", "run", "generated.go", input)
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

	cleanup, err := generateRunnable(t, match, "int", map[string]string{
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

// TestNoState tests matching a single string, no state machine required.
func TestNoState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compiled tests in short mode")
	}

	cleanup, err := generateRunnable(t, match, "int", map[string]string{
		"foo": "1",
	}, "0")
	defer cleanup()
	if err != nil {
		t.Fatalf(err.Error())
	}

	expectMatch(t, "foo", "1")
}

// TestInsensitive tests a case-insensitive matcher.
func TestInsensitive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compiled tests in short mode")
	}

	cleanup, err := generateRunnable(t, match, "int", map[string]string{
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

	cleanup, err := generateRunnable(t, match, "int", map[string]string{
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

	cleanup, err := generateRunnable(t, match, "int", map[string]string{
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

	cleanup, err := generateRunnable(t, match, "int", map[string]string{
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

// TestChained tests chaining multiple state machines together, to match
// longer strings.
func TestChained(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compiled tests in short mode")
	}

	oldMaxState := maxState
	defer func() { maxState = oldMaxState }()
	maxState = 16

	cleanup, err := generateRunnable(t, match, "int", map[string]string{
		"abcdef": "1",
		"ghijkl": "2",
	}, "0")
	defer cleanup()
	if err != nil {
		t.Fatalf(err.Error())
	}

	expectMatch(t, "abcdef", "1")
	expectMatch(t, "ghijkl", "2")
	expectMatch(t, "123456", "0")
}

// TestReverse tests a simple reverse matcher.
func TestReverse(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compiled tests in short mode")
	}

	cleanup, err := generateRunnable(t, reverseMatch, "string", map[string]string{
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
	if err := GenerateTest(f, "Match", "", map[string]string{"a": "1"}); err == nil {
		t.Errorf("no error from GenerateTest (forward matcher) on closed io.Writer")
	}
	if err := GenerateTest(f, "", "MatchReverse", map[string]string{"a": "1"}); err == nil {
		t.Errorf("no error from GenerateTest (reverse matcher) on closed io.Writer")
	}
}
