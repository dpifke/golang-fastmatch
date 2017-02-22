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
	"hash/fnv"
	"io"
	"sort"
	"strconv"
)

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
// The output is not buffered, and will be incomplete if an error is
// returned.  If the caller cares about this, they should have a way to
// discard the written output on error.  Errors writing to the supplied
// io.Writer will be passed back to the caller.
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
	equiv := makeEquivalents(flags...)
	var stop, ignore, ignoreExcept []rune

	partialMatch := false
	backwards := false
	for _, flag := range flags {
		if flag == HasPrefix {
			if backwards {
				return &ErrBadFlags{cannotCombine: []string{"HasPrefix", "HasSuffix"}}
			}
			partialMatch = true
		} else if flag == HasSuffix {
			if partialMatch && !backwards {
				return &ErrBadFlags{cannotCombine: []string{"HasPrefix", "HasSuffix"}}
			}
			partialMatch = true
			backwards = true
		}
		if len(flag.stop) > 0 {
			stop = append(stop, flag.stop...)
		}
		if len(flag.ignore) > 0 {
			if len(ignoreExcept) > 0 {
				return &ErrBadFlags{cannotCombine: []string{"Ignore", "IgnoreExcept"}}
			}
			ignore = append(ignore, flag.ignore...)
		}
		if len(flag.ignoreExcept) > 0 {
			if len(ignore) > 0 {
				return &ErrBadFlags{cannotCombine: []string{"Ignore", "IgnoreExcept"}}
			}
			ignoreExcept = append(ignoreExcept, flag.ignoreExcept...)
		}
	}

	// Check that stop and ignore runes are never equivalent.
	var stopIgnore sortableRunes
	for _, r1 := range stop {
		for _, r2 := range ignore {
			if equiv.isEquiv(r1, r2) {
				stopIgnore = append(stopIgnore, r1)
			}
		}
	}
	if len(stopIgnore) > 0 {
		return &ErrBadFlags{cannotStopIgnore: stopIgnore}
	}

	stop = equiv.expand(stop)
	ignore = equiv.expand(ignore)
	ignoreExcept = equiv.expand(ignoreExcept)

	// Create a new map with the actual keys being searched for.  If stop
	// runes were specified, the keys will be truncated if they contain
	// the stop character.  If we're suffix matching, these will be in
	// reverse order, since we examine the string back-to-front.  For
	// purposes of error reporting, we also need to be able to map the
	// modified key back to the original.
	var cases map[string]string
	var backToOrig map[string][]string
	if len(stop) > 0 || len(ignore) > 0 || len(ignoreExcept) > 0 {
		cases = make(map[string]string, len(origCases))
		backToOrig = make(map[string][]string, len(origCases))

		for key, value := range origCases {
			if backwards {
				key = reverseString(key)
			}

			newKey := make([]rune, 0, len(key))
		mangleKey:
			for _, r1 := range key {
				for _, r2 := range stop {
					if r1 == r2 {
						break mangleKey
					}
				}
				if len(ignoreExcept) > 0 {
					notIgnored := false
					for _, r2 := range ignoreExcept {
						if r1 == r2 {
							notIgnored = true
							break
						}
					}
					if !notIgnored {
						continue mangleKey
					}
				} else {
					for _, r2 := range ignore {
						if r1 == r2 {
							continue mangleKey
						}
					}
				}
				newKey = append(newKey, r1)
			}
			cases[string(newKey)] = value
			backToOrig[string(newKey)] = append(backToOrig[string(newKey)], key)
		}
	} else if backwards {
		cases = make(map[string]string, len(origCases))
		backToOrig = make(map[string][]string, len(origCases))

		for key, value := range origCases {
			newKey := reverseString(key)
			cases[newKey] = value
			backToOrig[newKey] = append(backToOrig[newKey], key)
		}
	} else {
		cases = origCases
	}

	// In order to generate (hopefully) unique labels, we hash the keys.
	h := fnv.New32a()

	// Search is partitioned based on the length of the input.  Split
	// cases into each possible search space:
	keys := make(map[int][]string)
	for key := range cases {
		keys[len(key)] = append(keys[len(key)], key)
		h.Write([]byte(key))
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

	inputAtOffset := func(off int) string {
		if backwards {
			if len(ignore) == 0 && len(ignoreExcept) == 0 {
				return fmt.Sprintf("input[len(input)-%d]", off+1)
			}
			return fmt.Sprintf("input[len(input)-%d-ignored]", off+1)
		}
		if len(ignore) == 0 && len(ignoreExcept) == 0 {
			return fmt.Sprintf("input[%d]", off)
		}
		return fmt.Sprintf("input[%d+ignored]", off)
	}

	wroteSwitch := false
	for _, l := range lengths {
		state := newStateMachine(keys[l])
		state.indexKeys(equiv, partialMatch)
		if err := state.checkAmbiguity(cases, origCases, backToOrig); err != nil {
			return err
		}

		// We don't bother checking the fmt.Fprint return value
		// everywhere, but we do want to do so once early on, so we
		// can bail if our effort is going to waste.  We also check it
		// on the final write, to make sure our io.Writer is still
		// good.
		if partialMatch || len(stop) > 0 || len(ignore) > 0 || len(ignoreExcept) > 0 {
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
		if len(ignore) > 0 || len(ignoreExcept) > 0 {
			fmt.Fprintln(w, "\t\tvar ignored int")
		}

		for realOffset := 0; realOffset < l; realOffset++ {
			if state.continued != nil && state.continued.offset == realOffset {
				fmt.Fprintln(w, "\t\tswitch state {")
				for before, after := range state.continued.collapsed {
					fmt.Fprintf(w, "\t\tcase %s:", before)
					fmt.Fprintln(w)
					fmt.Fprintf(w, "\t\t\tstate = 0x%x", after)
					fmt.Fprintln(w)
				}
				fmt.Fprintln(w, "\t\t}")
				state = state.continued
			}

			offset := realOffset - state.offset

			label := fmt.Sprintf("fastmatch_%x_l%d_o%d", h.Sum32(), l, realOffset)
			writeIgnore := func(w io.Writer) {
				fmt.Fprintf(w, "\t\t\tif len(input) <= ignored+%d {", l)
				fmt.Fprintln(w)
				fmt.Fprintln(w, "\t\t\t\treturn", none)
				fmt.Fprintln(w, "\t\t\t}")
				fmt.Fprintln(w, "\t\t\tignored++")
				fmt.Fprintln(w, "\t\t\tgoto", label)
			}
			if len(ignore) > 0 || len(ignoreExcept) > 0 {
				fmt.Fprintln(w, "\t"+label+":")
			}

			fmt.Fprintln(w, "\t\tswitch", inputAtOffset(realOffset), "{")

			if len(ignore) > 0 {
				fmt.Fprintf(w, "\t\tcase %s:", quoteRunes(ignore))
				fmt.Fprintln(w)
				writeIgnore(w)
			}

			for _, r := range state.possible[offset] {
				fmt.Fprintf(w, "\t\tcase %s:", quoteRunes(equiv.lookup(r)))
				fmt.Fprintln(w)

				if len(state.noMore[offset][r]) > 0 {
					fmt.Fprintln(w, "\t\t\tswitch state {")
					for _, key := range state.noMore[offset][r] {
						fmt.Fprintf(w, "\t\t\tcase %s:", state.finalString(key))
						fmt.Fprintln(w)
						fmt.Fprintln(w, "\t\t\t\treturn", cases[key])
					}
					fmt.Fprintln(w, "\t\t\t}")
				}

				if state.changes[offset][r] != 0 {
					fmt.Fprintf(w, "\t\t\tstate += 0x%x", state.changes[offset][r])
					fmt.Fprintln(w)
				}
			}
			if len(ignoreExcept) > 0 {
				// If a non-ignored rune is not present in any
				// of the matches at this position, finding it
				// in the input causes matching to cease:
				notInInput := equiv.expand(ignoreExcept, state.possible[offset], stop)
				if len(notInInput) > 0 {
					fmt.Fprintf(w, "\t\tcase %s:", quoteRunes(notInInput))
					fmt.Fprintln(w)
					fmt.Fprintln(w, "\t\t\treturn", none)
				}

				// Ignore all other runes:
				fmt.Fprintln(w, "\t\tdefault:")
				writeIgnore(w)
			} else if !partialMatch || realOffset != l-1 {
				// Any rune not present in any of the matches
				// at this position causes matching to cease.
				//
				// (We can omit this on the last rune when
				// partial matching, since we've already
				// omitted our final switch block and the next
				// statement will be a return none.)
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
			if l != lengths[len(lengths)-1] {
				// We can omit this if we're at the end of the
				// function.
				fmt.Fprintln(w, "\t\treturn", none)
			}
			fmt.Fprintln(w, "\t}") // end of "if len(input)"
		} else {
			// If we relaxed our original length check due to
			// StopUpon, Ignore, or IgnoreExcept flags, consume
			// any remaining ignored runes and check that the
			// string either terminates here or the next character
			// is a stop character.
			label := fmt.Sprintf("fastmatch_%x_l%d_final", h.Sum32(), l)
			if len(ignore) > 0 || len(ignoreExcept) > 0 {
				fmt.Fprintln(w, "\t"+label+":")
				fmt.Fprintf(w, "\t\tif len(input) > %d+ignored {", l)
				fmt.Fprintln(w)
			} else if len(stop) > 0 {
				fmt.Fprintf(w, "\t\tif len(input) > %d {", l)
				fmt.Fprintln(w)
			}
			if len(ignore) > 0 || len(ignoreExcept) > 0 || len(stop) > 0 {
				fmt.Fprintln(w, "\t\t\tswitch", inputAtOffset(l), "{")
				if len(stop) > 0 {
					fmt.Fprintf(w, "\t\t\tcase %s:", quoteRunes(stop))
					fmt.Fprintln(w)
					// empty case
				}
				if len(ignore) > 0 || len(ignoreExcept) > 0 {
					if len(ignore) > 0 {
						fmt.Fprintf(w, "\t\t\tcase %s:", quoteRunes(ignore))
						fmt.Fprintln(w)
					} else {
						fmt.Fprintf(w, "\t\t\tcase %s:", quoteRunes(equiv.expand(ignoreExcept, stop)))
						fmt.Fprintln(w)
						fmt.Fprintln(w, "\t\t\t\treturn", none)
						fmt.Fprintln(w, "\t\t\tdefault:")
					}
					fmt.Fprintln(w, "\t\t\t\tignored++")
					fmt.Fprintln(w, "\t\t\t\tgoto", label)
				}
				if len(ignoreExcept) == 0 {
					fmt.Fprintln(w, "\t\t\tdefault:")
					fmt.Fprintln(w, "\t\t\t\treturn", none)
				}
				fmt.Fprintln(w, "\t\t\t}") // end of "switch input[l]"
				fmt.Fprintln(w, "\t\t}")   // end of "if len(input) > l"
			}

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
			if len(stop) > 0 || len(ignore) > 0 || len(ignoreExcept) > 0 {
				fmt.Fprintln(w, "\t}") // end of "if len(input)"
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
	if err := checkReverseAmbiguity(cases); err != nil {
		return err
	}

	// Case statements are written in alphabetic order by key
	keys := make([]string, 0, len(cases))
	for key := range cases {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	if _, err := fmt.Fprintln(w, "\tswitch input {"); err != nil {
		return err
	}
	for _, key := range keys {
		fmt.Fprintf(w, "\tcase %s:", cases[key])
		fmt.Fprintln(w)
		fmt.Fprintln(w, "\t\treturn", strconv.Quote(key))
	}
	fmt.Fprintln(w, "\tdefault:")
	fmt.Fprintln(w, "\t\treturn", none)
	fmt.Fprintln(w, "\t}") // end of switch

	_, err := fmt.Fprintln(w, "}") // end of func
	return err
}

// GenerateTest outputs a simple unit test which exercises the generated code.
//
// An error is returned if the supplied io.Writer is not valid.  As with
// Generate and GenerateReverse, the caller is expected to write the method
// signature (with a *testing.T argument named t) before calling this
// function.
//
// fn and reverseFn should be the fmt.Printf-style format strings accepting a
// single argument, which will be replaced with the test input for the
// generated function.  This is typically something like "Function(%q)" for
// the matcher and "%s.String()" for the reverse matcher.  Passing "" causes
// the respective function to not be tested.
//
// Flags should match what was passed to Generate, but are currently ignored.
// Future versions of this routine may output more sophisticated tests which
// take flags into account.
func GenerateTest(w io.Writer, fn, reverseFn string, cases map[string]string, _ ...*Flag) error {
	keys := make([]string, 0, len(cases))
	for key := range cases {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if fn != "" {
			_, err := fmt.Fprintf(w, "\tif %s != %s {", fmt.Sprintf(fn, key), cases[key])
			if err != nil {
				return err
			}
			fmt.Fprintln(w)
			fmt.Fprintf(w, "\t\tt.Errorf(\"wrong answer for %%q\", %q)", key)
			fmt.Fprintln(w)
			fmt.Fprintln(w, "\t}") // endif
		}

		if reverseFn != "" {
			_, err := fmt.Fprintf(w, "\tif %s != %q {", fmt.Sprintf(reverseFn, cases[key]), key)
			if err != nil {
				return err
			}
			fmt.Fprintln(w)

			// Escape opening and closing quotes if needed:
			s := cases[key]
			if s[0] == '"' {
				s = "\\" + s[:len(s)-1] + `\"`
			}

			fmt.Fprintf(w, "\t\tt.Errorf(\"wrong reverse answer for %s\")", s)
			fmt.Fprintln(w)
			fmt.Fprintln(w, "\t}") // endif
		}
	}
	_, err := fmt.Fprintln(w, "}") // end of func
	return err
}
