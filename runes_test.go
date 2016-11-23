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
	"reflect"
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
