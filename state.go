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
	"math"
)

// ErrOverflow is returned when the stateMachine runs out of space for storing
// intermediate states.  The resolution is to reduce the length and/or
// quantity of the input strings being matched.
var ErrOverflow = errors.New("too many values to match (uint64 overflow)")

// stateMachine holds the mapping between a match and the intermediate state
// changes (runes encountered) leading up to a match.
type stateMachine struct {
	next     uint64
	base     uint64
	final    map[string][]uint64
	possible [][]rune
	changes  []map[rune]uint64
	noMore   []map[rune][]string
}

// newStateMachine initializes a stateMachine.
func newStateMachine(keys []string) *stateMachine {
	state := &stateMachine{
		next:  1,
		base:  1,
		final: make(map[string][]uint64, len(keys)),
	}
	for _, key := range keys {
		state.final[key] = make([]uint64, 0, len(key))
	}
	return state
}

// shift should be called at each new position, to ensure new intermediate
// state values do not overlap with previous ones.
func (state *stateMachine) shift() {
	state.base = state.next
}

// increment creates a new intermediate state.  It returns an error if we have
// too many intermediate states to fit in a uint64.
func (state *stateMachine) increment() error {
	if state.base > math.MaxUint64-state.next {
		// TODO: we can work around this by creating an additional
		// stateMachine and chaining to it.  I have a MIME type parser
		// which must deal with abominations such as
		// "application/vnd.openxmlformats-officedocument.presentationml.commentAuthors+xml",
		// so I'll probably implement this sooner rather than later.
		return ErrOverflow
	}
	state.next += state.base
	return nil
}

// indexKeys assigns a unique state value to each possible state change.  For
// partial matching, this method also notes where the state should be checked
// against possible final values.
func (state *stateMachine) indexKeys(equiv runeEquivalents, partialMatch bool) error {
	longestKey := 0
	keys := make([]string, 0, len(state.final))
	for key := range state.final {
		keys = append(keys, key)
		if len(key) > longestKey {
			longestKey = len(key)
		}
	}

	needShift := true
	state.possible = make([][]rune, longestKey)
	state.changes = make([]map[rune]uint64, longestKey)
	state.noMore = make([]map[rune][]string, longestKey)
	for offset := 0; offset < longestKey; offset++ {
		state.possible[offset] = equiv.uniqueAtOffset(keys, offset)

		if len(state.possible[offset]) > 1 {
			if needShift {
				state.shift()
				needShift = false
			}

			state.changes[offset] = make(map[rune]uint64, len(keys))
			for _, r := range state.possible[offset] {
				needIncr := false
				for _, key := range keys {
					if partialMatch && offset >= len(key)-1 {
						continue
					}
					if equiv.isEquiv(rune(key[offset]), r) {
						state.final[key] = append(state.final[key], state.next)
						needIncr = true
					}
				}
				if needIncr {
					state.changes[offset][r] = state.next
					if err := state.increment(); err != nil {
						return err
					}
					needShift = true
				}
			}
		}

		state.noMore[offset] = make(map[rune][]string, len(state.possible[offset]))
		if partialMatch {
			for _, r := range state.possible[offset] {
				for _, key := range keys {
					if len(key)-1 == offset && equiv.isEquiv(rune(key[offset]), r) {
						state.noMore[offset][r] = append(state.noMore[offset][r], key)
					}
				}
			}
		}
	}

	return nil
}

// deleteKey forgets about a possible match.  This is called by checkAmbiguity
// to prune redundant keys, so that we don't output duplicate or unreachable
// case statements.
func (state *stateMachine) deleteKey(key string) {
	delete(state.final, key)

	for _, noMore := range state.noMore {
		for r := range noMore {
			for n := range noMore[r] {
				if noMore[r][n] == key {
					if n < len(noMore[r])-1 {
						copy(noMore[r][n:], noMore[r][n+1:])
					}
					noMore[r] = noMore[r][:len(noMore[r])-1]
					if n >= len(noMore[r])-1 {
						break
					}
				}
			}
		}
	}
}

// finalString returns a string representing the final state of each key.  To
// make the generated code slightly more readable, this consists of an
// expression summing each intermediate state value (in hex).
func (state *stateMachine) finalString(key string) string {
	if len(state.final[key]) == 0 {
		return "0"
	}

	var b bytes.Buffer
	for n, value := range state.final[key] {
		if n != 0 {
			b.WriteString(" + ")
		}
		b.WriteString(fmt.Sprintf("0x%x", value))
	}
	return b.String()
}
