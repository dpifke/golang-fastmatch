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
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
)

// ErrBadFlags is returned when nonsensical flags are passed to Generate.
var ErrBadFlags = errors.New("HasPrefix and HasSuffix flags are mutually exclusive")

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

// reverseString returns a string in reverse order.  I'm shocked this isn't
// part of the standard library.
func reverseString(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// Generate outputs Go code to compare a string to a set of possible matches
// which are known at compile-time.
//
// Each entry in the map consists of a possible match as the key, and the
// corresponding expression to return as the value.  none is the expression to
// return if no match is found.
//
// Code to perform the match is written to the supplied io.Writer.  Before
// calling this function, the caller is expected to write the method signature
// and any input pre-processing logic.  The string to examine should be in a
// variable named "input".
//
// If flags are specified, it's possible to generate ambiguous code, in which
// the same input string will match multiple entries in the cases map, with
// different return values.  This function attempts to detect this and will
// return an error if ambiguity is detected.
//
// An error is also returned if the provided io.Writer is invalid, or if there
// are too many matches to fit within our uint64 state machine.
//
// The output is not buffered, and will be incomplete if an error is
// returned.  If the caller cares about this, they should have a way to
// discard the written output on error.
//
// Example usage:
//
//	fmt.Fprintln(w, "func matchFoo(input string) int {")
//	fastmatch.Generate(w, map[string]string{
//		"foo": "1",
//		"bar": "2",
//		"baz": "3",
//	}, "-1", fastmatch.Insensitive)
func Generate(w io.Writer, origCases map[string]string, none string, flags ...*Flag) error {
	equiv := makeRuneEquivalents(flags...)
	partialMatch := false
	backwards := false
	for _, flag := range flags {
		if flag == HasPrefix {
			if backwards {
				return ErrBadFlags
			}
			partialMatch = true
		} else if flag == HasSuffix {
			if partialMatch && !backwards {
				return ErrBadFlags
			}
			partialMatch = true
			backwards = true
		}
	}

	// For backwards matching (HasSuffix), reverse the order of the
	// strings being searched for:
	var cases map[string]string
	if backwards {
		cases = make(map[string]string, len(origCases))
		for key, value := range origCases {
			cases[reverseString(key)] = value
		}
	} else {
		cases = origCases
	}

	// Search is partitioned based on the length of the input.  Split
	// cases into each possible search space:
	keys := make(map[int][]string)
	for key := range cases {
		keys[len(key)] = append(keys[len(key)], key)
	}
	lengths := sort.IntSlice(make([]int, 0, len(keys)))
	for len := range keys {
		lengths = append(lengths, len)
	}
	sort.Sort(sort.Reverse(lengths))

	// For partial matching, include shorter cases in the search space for
	// longer ones.  (Reminder: lengths array is sorted in descending
	// order.)
	if partialMatch {
		for i := len(lengths) - 1; i > 0; i-- {
			smaller := lengths[i]
			bigger := lengths[i-1]
			keys[bigger] = append(keys[bigger], keys[smaller]...)
		}
	}

	wroteSwitch := false
	for _, l := range lengths {
		state := newStateMachine(keys[l])
		if err := state.indexKeys(equiv, partialMatch); err != nil {
			return err
		}
		if err := state.checkAmbiguity(cases, backwards); err != nil {
			return err
		}

		// We don't bother checking the fmt.Fprint return value
		// everywhere, but we do want to do so once early on, so we
		// can bail if our effort is going to waste.  We also check it
		// on the final write, to make sure our io.Writer is still
		// good.
		if partialMatch {
			if _, err := fmt.Fprintf(w, "\tif len(input) >= %d {", l); err != nil {
				return err
			}
		} else {
			if !wroteSwitch {
				fmt.Fprintln(w, "\tswitch len(input) {")
				wroteSwitch = true
			}
			if _, err := fmt.Fprintf(w, "\tcase %d:", l); err != nil {
				return err
			}
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w, "\t\tvar state uint64")

		for offset := 0; offset < l; offset++ {
			if backwards {
				fmt.Fprintf(w, "\t\tswitch input[len(input)-%d] {", offset+1)
			} else {
				fmt.Fprintf(w, "\t\tswitch input[%d] {", offset)
			}
			fmt.Fprintln(w)
			for _, r := range state.possible[offset] {
				fmt.Fprintf(w, "\t\tcase %s:", equiv.lookupString(r))
				fmt.Fprintln(w)

				if len(state.noMore[offset][r]) == 1 {
					fmt.Fprintln(w, "\t\t\treturn", cases[state.noMore[offset][r][0]])
				} else if len(state.noMore[offset][r]) > 0 {
					fmt.Fprintln(w, "\t\t\tswitch state {")
					for _, key := range state.noMore[offset][r] {
						fmt.Fprintf(w, "\t\t\tcase %s:", state.finalString(key))
						fmt.Fprintln(w)
						fmt.Fprintln(w, "\t\t\t\treturn", cases[key])
					}
					fmt.Fprintln(w, "\t\t\t}")
				}

				if state.changes[offset][r] > 0 {
					fmt.Fprintf(w, "\t\t\tstate += 0x%x", state.changes[offset][r])
					fmt.Fprintln(w)
				}
			}
			if !partialMatch || offset != l-1 {
				fmt.Fprintln(w, "\t\tdefault:")
				fmt.Fprintln(w, "\t\t\treturn", none)
			}
			fmt.Fprintln(w, "\t\t}") // end of "switch input[offset]"
		}

		if state.next == 1 {
			// Prevent compiler from complaining:
			fmt.Fprintln(w, "\t\t_ = state")
		}

		if partialMatch {
			// Final switch block has already been emitted.
			fmt.Fprintln(w, "\t\treturn", none)
			fmt.Fprintln(w, "\t}") // end of "if len(input)"
		} else {
			// Compare actual state to possible final values:
			if len(state.final) == 1 && state.next == 1 {
				for key := range state.final {
					fmt.Fprintln(w, "\t\treturn", cases[key])
				}
			} else {
				fmt.Fprintln(w, "\t\tswitch state {")
				for key := range state.final {
					fmt.Fprintf(w, "\t\tcase %s:", state.finalString(key))
					fmt.Fprintln(w)
					fmt.Fprintln(w, "\t\t\treturn", cases[key])
				}
				fmt.Fprintln(w, "\t\t}")
			}
		}
	}
	if wroteSwitch {
		fmt.Fprintln(w, "\t}") // end of "switch len(input)"
	}
	fmt.Fprintln(w, "\treturn", none)
	_, err := fmt.Fprintln(w, "}") // end of func
	return err
}

// GenerateReverse outputs Go code that returns the string value for a given
// match.  The result from the generated function will be the reverse of that
// from a function generated with Generate.
//
// If the supplied io.Writer is not valid, or if more than one string maps to
// the same value, an error is returned.
//
// This function accepts flags (in order to match Generate's function
// signature), but they are currently ignored.
func GenerateReverse(w io.Writer, cases map[string]string, none string, _ ...*Flag) error {
	keys := make([]string, 0, len(cases))
	for key := range cases {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	if _, err := fmt.Fprintln(w, "\tswitch input {"); err != nil {
		return err
	}
	ambiguity := make(map[string][]string, len(cases))
	for _, key := range keys {
		ambiguity[cases[key]] = append(ambiguity[cases[key]], key)
		fmt.Fprintf(w, "\tcase %s:", cases[key])
		fmt.Fprintln(w)
		fmt.Fprintln(w, "\t\treturn", strconv.Quote(key))
	}

	var b bytes.Buffer
	for _, returnValues := range ambiguity {
		if len(returnValues) == 1 {
			continue
		}

		if b.Len() == 0 {
			b.WriteString("ambiguous values: ")
		} else {
			b.Write([]byte{';', ' '})
		}
		first := true
		for _, value := range returnValues {
			if !first {
				b.Write([]byte{',', ' '})
			} else {
				first = false
			}
			b.WriteString(strconv.Quote(value))
		}
	}
	if b.Len() != 0 {
		return errors.New(b.String())
	}

	fmt.Fprintln(w, "\tdefault:")
	fmt.Fprintln(w, "\t\treturn", none)
	fmt.Fprintln(w, "\t}")
	fmt.Fprintln(w, "}")

	return nil
}
