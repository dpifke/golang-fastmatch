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
	"strconv"
)

// checkAmbiguity verifies there is exactly one possible return value for each
// final state, returning an error if any matches are ambiguous.
func (state *stateMachine) checkAmbiguity(cases map[string]string, backwards bool) error {
	// map[final state] -> map[final rune] -> map[return value] -> map[key] -> true
	ambiguity := make(map[uint64]map[rune]map[string]map[string]bool, len(cases))
	for key, values := range state.final {
		var sum uint64
		for _, value := range values {
			sum += value
		}

		// Look in state.noMore for this key; the final rune may be
		// distinct, but doesn't have a state value assigned to it.
		var nm rune
		for r, keys := range state.noMore[len(key)-1] {
			for _, k := range keys {
				if k == key {
					nm = r
					break
				}
			}
		}

		if _, exists := ambiguity[sum]; !exists {
			ambiguity[sum] = make(map[rune]map[string]map[string]bool)
		}
		if _, exists := ambiguity[sum][nm]; !exists {
			ambiguity[sum][nm] = make(map[string]map[string]bool)
		}
		if _, exists := ambiguity[sum][nm][cases[key]]; !exists {
			ambiguity[sum][nm][cases[key]] = make(map[string]bool)
		}
		ambiguity[sum][nm][cases[key]][key] = true

		if nm != 0 {
			// Add longer keys which share the same final rune and
			// intermediate state.
			for other, values := range state.final {
				if len(other) <= len(key) {
					continue // not longer
				}

				var otherSum uint64
				if len(values) >= len(key) {
					if values[len(key)-1] != state.changes[len(key)-1][nm] {
						continue // different final rune
					}

					for n := 0; n < len(key)-1; n++ {
						otherSum += values[n]
					}
				} else {
					for _, value := range values {
						otherSum += value
					}
				}

				if otherSum != sum {
					continue // different intermediate state
				}

				if _, exists := ambiguity[sum][nm][cases[other]]; !exists {
					ambiguity[sum][nm][cases[other]] = make(map[string]bool)
				}
				ambiguity[sum][nm][cases[other]][other] = true
			}
		}
	}

	var b bytes.Buffer
	for _, finalChars := range ambiguity {
		for _, returnValues := range finalChars {
			if len(returnValues) == 1 {
				// Not ambiguous, but delete all but the
				// shortest key that maps to this return
				// value.  This eliminates duplicate and
				// unreachable case statements.
				var returnValue string
				for returnValue = range returnValues {
				}
				var shortest string
				for key := range returnValues[returnValue] {
					if shortest == "" || len(key) < len(shortest) {
						shortest = key
					}
				}
				for key := range returnValues[returnValue] {
					if key != shortest {
						state.deleteKey(key)
					}
				}

				continue
			}

			if b.Len() == 0 {
				b.WriteString("ambiguous matches: ")
			} else {
				b.Write([]byte{';', ' '})
			}
			first := true
			for _, keys := range returnValues {
				for key := range keys {
					if backwards {
						key = reverseString(key)
					}
					if !first {
						b.Write([]byte{',', ' '})
					} else {
						first = false
					}
					b.WriteString(strconv.Quote(key))
				}
			}
		}
	}

	if b.Len() == 0 {
		return nil
	}
	return errors.New(b.String())
}
