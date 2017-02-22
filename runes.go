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

// sortableRunes implements sort.Sortable on a slice of runes.
type sortableRunes []rune

func (r sortableRunes) Len() int           { return len(r) }
func (r sortableRunes) Swap(a, b int)      { r[a], r[b] = r[b], r[a] }
func (r sortableRunes) Less(a, b int) bool { return r[a] < r[b] }

// runeEquivalents holds our map of which runes are equivalent to each other.
type runeEquivalents map[rune]sortableRunes

// dedupedRuneEquivalents is used internally in the construction of
// runeEquivalents.
type dedupedRuneEquivalents map[rune]map[rune]bool

// set adds a rune key and zero or more values to dedupedRuneEquivalents,
// calling make() on the second-level map as needed.
func (equiv dedupedRuneEquivalents) set(r rune, rs ...rune) {
	if _, exists := equiv[r]; !exists {
		equiv[r] = make(map[rune]bool, len(rs)+1)
		equiv[r][r] = true
	}
	for _, r2 := range rs {
		equiv[r][r2] = true
	}
}

// collapse converts dedupedRuneEquivalents to runeEquivalents.
func (equiv dedupedRuneEquivalents) collapse() runeEquivalents {
	newEquiv := make(runeEquivalents, len(equiv))
	for r1, rm := range equiv {
		// If equiv['a'] contains 'b', and equiv['b'] contains 'c', we
		// want to ensure 'c' is present in equiv['a'].  This requires
		// several passes, until we've determined the transitive
		// equivalence of every rune therein.
		seen := make(map[rune]bool, len(rm))
	populateTransience:
		for {
			for r2 := range rm {
				if seen[r2] {
					continue
				}
				for r3 := range equiv[r2] {
					equiv[r1][r3] = true
				}
				seen[r2] = true
				continue populateTransience
			}
			break
		}

		// Now we can convert from map[rune]bool to sorted []rune.
		newEquiv[r1] = make(sortableRunes, 0, len(rm))
		for r2 := range rm {
			newEquiv[r1] = append(newEquiv[r1], r2)
		}
		sort.Sort(newEquiv[r1])
	}
	return newEquiv
}

// makeEquivalents builds our rune equivalence map based on flags.
func makeEquivalents(flags ...*Flag) runeEquivalents {
	equiv := make(dedupedRuneEquivalents)

	for _, f := range flags {
		if f == Insensitive {
			for lower := 'a'; lower <= 'z'; lower++ {
				upper := 'A' + (lower - 'a')
				equiv.set(lower, upper)
				equiv.set(upper, lower)
			}
		} else if f == Normalize {
			continue // TODO: not yet implemented
		} else if len(f.equivalent) > 0 {
			for _, r := range f.equivalent {
				equiv.set(r, f.equivalent...)
			}
		}
	}

	return equiv.collapse()
}

// lookup returns a map entry from runeEquivalents, defaulting to a slice
// containing just the lookup key if there are no equivalents for that rune.
func (equiv runeEquivalents) lookup(r rune) []rune {
	if rs, found := equiv[r]; found {
		return rs
	}
	return []rune{r}
}

// expand returns a sorted, de-duped slice of runes (including equivalents)
// from rs.  (This is similar to lookup, except it operates on a slice instead
// of an individual rune.)
//
// Zero or more slices of runes (including equivalents) to exclude from the
// output can be also specified.
func (equiv runeEquivalents) expand(rs []rune, exclude ...[]rune) []rune {
	rm := make(map[rune]bool, len(rs))
findExclusions:
	for _, r1 := range rs {
		for _, excluded := range exclude {
			for _, r2 := range excluded {
				if equiv.isEquiv(r1, r2) {
					continue findExclusions
				}
			}
		}
		for _, r2 := range equiv.lookup(r1) {
			rm[r2] = true
		}
	}

	newRs := make(sortableRunes, 0, len(rm))
	for r := range rm {
		newRs = append(newRs, r)
	}
	sort.Sort(newRs)

	return newRs
}

// quoteRunes formats a slice of runes for use in a case statement.
func quoteRunes(runes []rune) string {
	var b bytes.Buffer
	for _, r := range runes {
		if b.Len() != 0 {
			b.Write([]byte{',', ' '})
		}
		b.WriteString(strconv.QuoteRuneToASCII(r))
	}
	return b.String()
}

// isEquiv returns true if two runes are equivalent.
func (equiv runeEquivalents) isEquiv(r1, r2 rune) bool {
	for _, r := range equiv.lookup(r1) {
		if r == r2 {
			return true
		}
	}
	return false
}

// uniqueAtOffset returns a sorted list (sans duplicates) of possible runes at
// a given offset for a given set of keys.
func (equiv runeEquivalents) uniqueAtOffset(keys []string, offset int) []rune {
	runes := sortableRunes(make([]rune, 0, len(keys)))
	seen := make(map[rune]bool, len(keys))
possibilities:
	for _, key := range keys {
		if len(key) > offset {
			r := rune(key[offset])
			for _, r2 := range equiv.lookup(r) {
				if seen[r2] {
					continue possibilities
				}
				seen[r2] = true
			}
			runes = append(runes, r)
		}
	}
	sort.Sort(runes)

	return runes
}
