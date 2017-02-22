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
	"sort"
	"strconv"
)

// ErrAmbiguous is returned when Generate or GenerateReverse is passed
// ambiguous matches: cases where it's possible for the same input string to
// match different return values.
type ErrAmbiguous struct {
	keys []map[string]bool
}

// add provides one or more keys that are ambiguous with each other.
func (e *ErrAmbiguous) add(backToOrig map[string][]string, keys ...string) {
	origKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		if k, found := backToOrig[key]; found {
			origKeys = append(origKeys, k...)
		} else {
			origKeys = append(origKeys, key)
		}
	}

	for n := range e.keys {
		for existing := range e.keys[n] {
			for _, other := range origKeys {
				if existing == other {
					for _, key := range origKeys {
						e.keys[n][key] = true
					}
					return
				}
			}
		}
	}

	e.keys = append(e.keys, make(map[string]bool, len(origKeys)))
	for _, key := range origKeys {
		e.keys[len(e.keys)-1][key] = true
	}
}

// sliceOfStringSlices implements strings.Sortable on a slice of string
// slices.  The first-level slice is sorted according to the first element in
// each second-level slice.
type sliceOfStringSlices [][]string

func (s sliceOfStringSlices) Len() int      { return len(s) }
func (s sliceOfStringSlices) Swap(a, b int) { s[a], s[b] = s[b], s[a] }

func (s sliceOfStringSlices) Less(a, b int) bool {
	if len(s[a]) == 0 && len(s[b]) > 0 {
		return true
	} else if len(s[b]) == 0 {
		return false
	}
	return s[a][0] < s[b][0]
}

// sort reorders elements in the proper order.
func (s sliceOfStringSlices) sort() {
	for n := range s {
		sort.Strings(s[n])
	}
	sort.Sort(s)
}

// sortedKeys collapses our map of duplicate keys to nested slices, in
// lexicographical order.
func (e *ErrAmbiguous) sortedKeys() [][]string {
	var keys sliceOfStringSlices
	for n, ambiguous := range e.keys {
		keys = append(keys, make([]string, 0, len(ambiguous)))
		for key := range ambiguous {
			keys[n] = append(keys[n], key)
		}
	}
	keys.sort()

	return keys
}

func (e *ErrAmbiguous) Error() string {
	var b bytes.Buffer
	for _, group := range e.sortedKeys() {
		if b.Len() == 0 {
			b.WriteString("ambiguous matches: ")
		} else {
			b.Write([]byte{';', ' '})
		}

		first := true
		for _, key := range group {
			if !first {
				b.Write([]byte{',', ' '})
			} else {
				first = false
			}
			b.WriteString(strconv.Quote(key))
		}
	}
	return b.String()
}

// seenCases stores a collection of unique return value/key mappings.
type seenCases map[string]map[string]bool

// collapse returns lists of unique return values and keys.
func (sc seenCases) collapse() ([]string, []string) {
	var rets, keys []string
	for ret, keysMap := range sc {
		rets = append(rets, ret)
		for key := range keysMap {
			keys = append(keys, key)
		}
	}

	return rets, keys
}

// disambiguate maps final state and rune to a seenCases collection.
type disambiguate struct {
	cases map[uint64]map[rune]seenCases
	keys  map[string]bool
}

// foreach iterates over unique final states, calling the supplied function
// with the possible return values and keys for each.
func (d *disambiguate) foreach(f func([]string, []string)) {
	for sum := range d.cases {
		for r := range d.cases[sum] {
			rets, keys := d.cases[sum][r].collapse()
			f(rets, keys)
		}
	}
}

// add indexes a possible final state.
func (d *disambiguate) add(sum uint64, r rune, ret, key string) {
	if d.cases == nil {
		d.cases = make(map[uint64]map[rune]seenCases)
	}
	if _, exists := d.cases[sum]; !exists {
		d.cases[sum] = make(map[rune]seenCases)
	}
	if _, exists := d.cases[sum][r]; !exists {
		d.cases[sum][r] = make(seenCases)
	}
	if _, exists := d.cases[sum][r][ret]; !exists {
		d.cases[sum][r][ret] = make(map[string]bool)
	}
	d.cases[sum][r][ret][key] = true

	if d.keys == nil {
		d.keys = make(map[string]bool)
	}
	d.keys[key] = true
}

// indexNoMore iterates over the final states for partial matches in a
// stateMachine, adding them to the disambiguation index.
func (d *disambiguate) indexNoMore(state *stateMachine, cases map[string]string) {
	state.foreachNoMore(func(_ int, r rune, key string) {
		sum := state.finalState(key)
		d.add(sum, r, cases[key], key)

		// finalIdx is the index within state.final of our terminal
		// rune.  changesIdx is index within state.changes.
		var finalIdx, changesIdx int
		if state.offset == 0 {
			finalIdx = len(key) - 1
			changesIdx = finalIdx
		} else {
			// state.final[0] is the collapsed state from the
			// previous state machine.
			finalIdx = len(key) - state.offset
			changesIdx = finalIdx - 1
		}

		// Add longer keys which share the same final rune and
		// intermediate state:
		for other, values := range state.final {
			if len(other) <= len(key) {
				continue // not longer
			}

			if values[finalIdx] != state.changes[changesIdx][r] {
				continue // different final rune
			}

			var otherSum uint64
			for n := 0; n < finalIdx; n++ {
				otherSum += values[n]
			}
			if otherSum != sum {
				continue // different intermediate state
			}

			d.add(sum, r, cases[other], other)
		}
	})
}

// indexFinal iterates over the final states in a stateMachine.  States which
// were not already indexed by indexNoMore will be added to the disambiguation
// index.
func (d *disambiguate) indexFinal(state *stateMachine, cases map[string]string) {
	for key := range state.final {
		if d.keys[key] {
			continue // already indexed by indexNoMore()
		}
		d.add(state.finalState(key), 0, cases[key], key)
	}
}

// shortestString returns the shortest string from a list of strings.
func shortestString(ss []string) string {
	var shortest string
	for _, s := range ss {
		if shortest == "" || len(s) < len(shortest) {
			shortest = s
		}
	}
	return shortest
}

// checkAmbiguity verifies there is exactly one possible return value for each
// final state, returning an error if any matches are ambiguous.
func (state *stateMachine) checkAmbiguity(cases, origCases map[string]string, backToOrig map[string][]string) error {
	e := new(ErrAmbiguous)

	// Keys which got mangled or truncated to the same value (due to
	// StopUpon, Ignore, or IgnoreExcept) are caught first.
	for _, keys := range backToOrig {
		if len(keys) <= 1 {
			continue
		}
		rets := make(map[string]bool, len(keys))
		for _, key := range keys {
			rets[origCases[key]] = true
		}
		if len(rets) > 1 {
			// Only an issue if they have different return values.
			e.add(nil, keys...)
		}
	}

	// Now perform a more exhaustive search.
	for {
		d := new(disambiguate)
		d.indexNoMore(state, cases)
		if state.continued == nil {
			d.indexFinal(state, cases)
		}

		d.foreach(func(rets []string, keys []string) {
			if len(rets) == 1 {
				// Not ambiguous, but delete all but the
				// shortest key with this return value.  This
				// eliminates duplicate and unreachable case
				// statements.
				shortest := shortestString(keys)
				for _, key := range keys {
					if key != shortest {
						state.deleteKey(key)
					}
				}
			} else if len(keys) > 1 {
				e.add(backToOrig, keys...)
			}
		})

		if state.continued == nil {
			break
		} else {
			state = state.continued
		}
	}

	if len(e.keys) == 0 {
		return nil
	}
	return e
}

// checkReverseAmbiguity verifies that each return value maps to at most a
// single key.
//
// Having multiple keys return the same value is no problem for Generate, but
// causes duplicate/ambiguous case statements in GenerateReverse.
func checkReverseAmbiguity(cases map[string]string) error {
	d := make(map[string][]string, len(cases))
	for key := range cases {
		d[cases[key]] = append(d[cases[key]], key)
	}

	e := new(ErrAmbiguous)
	for _, keys := range d {
		if len(keys) > 1 {
			e.add(nil, keys...)
		}
	}

	if len(e.keys) == 0 {
		return nil
	}
	return e
}
