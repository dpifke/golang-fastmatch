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
	"fmt"
	"math"
)

// The maximum allowable state value.  Can be overridden for testing.
var maxState uint64 = math.MaxUint64

// stateMachine holds the mapping between a match and the intermediate state
// changes (runes encountered) leading up to a match.
type stateMachine struct {
	next      uint64
	base      uint64
	final     map[string][]uint64
	possible  [][]rune
	changes   []map[rune]uint64
	noMore    []map[rune][]string
	offset    int
	continued *stateMachine
	collapsed map[string]uint64
}

// foreachNoMore iterates over (length, final rune, key) tuples in the
// stateMachine.noMore map.
func (state *stateMachine) foreachNoMore(f func(int, rune, string)) {
	for len := range state.noMore {
		for r := range state.noMore[len] {
			for _, key := range state.noMore[len][r] {
				f(len, r, key)
			}
		}
	}
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

// makeNextStateMachine initializes an additional state machine once we've
// exceeded the number of intermediate states which fit in a uint64.
func (state *stateMachine) makeNextStateMachine(realOffset int) {
	offset := realOffset - state.offset
	if offset < 1 {
		// This should only be possible during testing, when maxState
		// != math.MaxUint64.
		panic("maxState too small")
	}

	// The current switch statement is incomplete, so truncate any
	// internal state learned on this pass.
	state.possible = state.possible[:offset]
	state.changes = state.changes[:offset]
	state.noMore = state.noMore[:offset]

	// Make a note of keys which finished at previous offsets; they don't
	// need to be copied to the next state machine.  (This will be
	// zero-length if we're not doing partial matching.)
	finishedKeys := make(map[string]bool, len(state.final))
	state.foreachNoMore(func(_ int, _ rune, key string) {
		finishedKeys[key] = true
	})

	// Now create the next state machine, copying remaining keys to it.
	state.continued = &stateMachine{
		next:      1,
		offset:    realOffset,
		final:     make(map[string][]uint64, len(state.final)-len(finishedKeys)),
		collapsed: make(map[string]uint64, len(state.final)-len(finishedKeys)),
	}
	for key := range state.final {
		if finishedKeys[key] {
			continue
		}

		// The current switch statement is incomplete, so forget any
		// intermediate state we've noted for this key.
		if state.offset == 0 {
			state.final[key] = state.final[key][:offset]
		} else {
			// Need to include initial value from previous
			// stateMachine.
			state.final[key] = state.final[key][:offset+1]
		}

		// The current sum gets "collapsed" into a new state value in
		// the next machine.  Note that many keys may share the same
		// intermediate state.
		before := state.finalString(key)
		after := state.continued.collapsed[before]
		if after == 0 {
			after = state.continued.next
			state.continued.next++
			state.continued.collapsed[before] = after
		}
		state.continued.final[key] = append(make([]uint64, 0, len(key)-realOffset+1), after)
	}
	state.continued.base = state.continued.next
}

// indexKeys assigns a unique state value to each possible state change.  For
// partial matching, this method also notes where the state should be checked
// against possible final values.
func (state *stateMachine) indexKeys(equiv runeEquivalents, partialMatch bool) {
	longestKey := 0
	keys := make([]string, 0, len(state.final))
	for key := range state.final {
		keys = append(keys, key)
		if len(key) > longestKey {
			longestKey = len(key)
		}
	}

	needShift := true
	state.possible = make([][]rune, longestKey-state.offset)
	state.changes = make([]map[rune]uint64, longestKey-state.offset)
	state.noMore = make([]map[rune][]string, longestKey-state.offset)
	for realOffset := state.offset; realOffset < longestKey; realOffset++ {
		offset := realOffset - state.offset

		state.possible[offset] = equiv.uniqueAtOffset(keys, realOffset)

		if len(state.possible[offset]) > 1 {
			if needShift {
				// This ensures new intermediate state values
				// do not overlap with previous ones.
				state.base = state.next
				needShift = false
			}

			state.changes[offset] = make(map[rune]uint64, len(keys))
			for _, r := range state.possible[offset] {
				needIncr := false
				for _, key := range keys {
					if partialMatch && realOffset >= len(key)-1 {
						continue
					}
					if equiv.isEquiv(rune(key[realOffset]), r) {
						state.final[key] = append(state.final[key], state.next)
						needIncr = true
					}
				}
				if needIncr {
					if state.base > maxState-state.next {
						state.makeNextStateMachine(realOffset)
						state.continued.indexKeys(equiv, partialMatch)
						return
					}
					state.changes[offset][r] = state.next
					state.next += state.base
					needShift = true
				}
			}
		} else {
			// All of the keys share the same rune at this offset,
			// so there's no state change.  However, we still need
			// to write something to each key's state.final, so
			// that offsets within that array match key offset.
			// The zeroes will be omitted by state.finalString().
			for _, key := range keys {
				state.final[key] = append(state.final[key], 0)
			}
		}

		state.noMore[offset] = make(map[rune][]string, len(state.possible[offset]))
		if partialMatch {
			for _, r := range state.possible[offset] {
				for _, key := range keys {
					if len(key)-1 == realOffset && equiv.isEquiv(rune(key[realOffset]), r) {
						state.noMore[offset][r] = append(state.noMore[offset][r], key)
					}
				}
			}
		}
	}
}

// remove removes a string from a slice of strings if present, in the same
// manner the delete builtin can remove a key from a map.
func remove(a []string, s string) []string {
	for n := 0; n < len(a); n++ {
		if a[n] == s {
			if n < len(a)-1 {
				copy(a[n:], a[n+1:])
			}
			a = a[:len(a)-1]
		}
	}

	if len(a) == 0 {
		return nil
	}
	return a
}

// deleteKey forgets about a possible match.  This is called by checkAmbiguity
// to prune redundant keys, so that we don't output duplicate or unreachable
// case statements.
func (state *stateMachine) deleteKey(key string) {
	delete(state.final, key)

	for _, noMore := range state.noMore {
		for r := range noMore {
			noMore[r] = remove(noMore[r], key)
		}
	}
}

// finalState returns the uint64 state value for a given key.
func (state *stateMachine) finalState(key string) (sum uint64) {
	for _, value := range state.final[key] {
		sum += value
	}
	return
}

// finalString returns a string representing the final state of each key.  To
// make the generated code slightly more readable, this consists of an
// expression summing each intermediate state value (in hex).
func (state *stateMachine) finalString(key string) string {
	var b bytes.Buffer
	for _, value := range state.final[key] {
		if value == 0 {
			continue
		}
		if b.Len() != 0 {
			b.WriteString(" + ")
		}
		b.WriteString(fmt.Sprintf("0x%x", value))
	}

	if b.Len() == 0 {
		return "0"
	}
	return b.String()
}
