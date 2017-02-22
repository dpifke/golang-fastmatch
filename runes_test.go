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

// TestRuneEquivalents tests the construction of runeEquivalents via the
// Equivalent flag (including sorting, de-duping, and transitivity).
func TestRuneEquivalents(t *testing.T) {
	equiv := makeEquivalents(
		Equivalent('c', 'b'),
		Equivalent('a', 'd'),
		Equivalent('a', 'c'),
		Equivalent('d', 'd', 'd', 'd'),
	)

	expect := []rune{'a', 'b', 'c', 'd'}
	for _, r := range expect {
		if !reflect.DeepEqual(expect, equiv.lookup(r)) {
			t.Errorf("expected %q, got %q looking up %q", expect, equiv.lookup(r), r)
		}
		if !equiv.isEquiv(r, 'a') {
			t.Errorf("%q should be equivalent to 'a'", r)
		}
	}

	// Also test the behavior of runes not in the equivalence set:
	if !reflect.DeepEqual([]rune{'.'}, equiv.lookup('.')) {
		t.Errorf("expected ['.'], got %q looking up '.'", equiv.lookup('.'))
	}
	if !equiv.isEquiv('.', '.') {
		t.Errorf("'.' should be equivalent to itself")
	}
	if equiv.isEquiv('.', 'a') {
		t.Error("'.' should not be equivalent to 'a'")
	}
}

// TestRuneEquivalents tests the construction of runeEquivalents via the
// Insensitive flag.
func TestRuneInsensitive(t *testing.T) {
	equiv := makeEquivalents(Insensitive)

	if !reflect.DeepEqual([]rune{'E', 'e'}, equiv.lookup('e')) {
		t.Errorf("expected ['E', 'e'], got %q looking up 'e'", equiv.lookup('e'))
	}
	if !equiv.isEquiv('E', 'e') {
		t.Error("'E' should be equivalent to 'e'")
	}
	if equiv.isEquiv('E', 'f') {
		t.Error("'E' should not be equivalent to 'f'")
	}
}

var equivalentExpandTests = []struct {
	args   []rune
	expect []rune
}{
	{[]rune{'a'}, []rune{'a', 'b'}},
	{[]rune{'b', 'a'}, []rune{'a', 'b'}},
	{[]rune{'b', 'a', 'c'}, []rune{'a', 'b', 'c'}},
	{[]rune{'b', 'c', 'a', 'b', 'c'}, []rune{'a', 'b', 'c'}},
	{[]rune{'c'}, []rune{'c'}},
	{[]rune{}, []rune{}},
}

// TestRuneEquivalentExpand tests expanding a list of runes to include
// equivalents.  The output list should be sorted and de-duped.
func TestRuneEquivalentExpand(t *testing.T) {
	equiv := makeEquivalents(Equivalent('a', 'b'))

	for _, testCase := range equivalentExpandTests {
		actual := equiv.expand(testCase.args)
		if !reflect.DeepEqual(testCase.expect, actual) {
			t.Errorf("expected %q, got %q", testCase.expect, actual)
		}
	}
}

// TestRuneEquivalentExpandExclude tests runeEquivalent.expand() with lists of
// runes to exclude.
func TestRuneEquivalentExpandExclude(t *testing.T) {
	equiv := makeEquivalents(Equivalent('a', 'b'), Equivalent('c', 'd'))

	actual := equiv.expand([]rune{'f', 'c', 'f', 'a', 'e'}, []rune{'b', 'c'})
	expect := []rune{'e', 'f'}
	if !reflect.DeepEqual(expect, actual) {
		t.Errorf("expected %q, got %q", expect, actual)
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
	equiv := makeEquivalents(Insensitive)

	for n := range expect {
		result := equiv.uniqueAtOffset(keys, n)
		if !reflect.DeepEqual(expect[n], result) {
			t.Errorf("expected %q, got %q at offset %d", expect[n], result, n)
		}
	}
}

var quoteRunesTests = []struct {
	input  []rune
	expect string
}{
	{[]rune{'a'}, "'a'"},
	{[]rune{'a', 'b'}, "'a', 'b'"},
	{[]rune{'a', 'b', 'c'}, "'a', 'b', 'c'"},
}

// TestQuoteRunes tests the output of the quoteRunes function.
func TestQuoteRunes(t *testing.T) {
	for _, testCase := range quoteRunesTests {
		if actual := quoteRunes(testCase.input); testCase.expect != actual {
			t.Errorf("expected %q, got %q", testCase.expect, actual)
		}
	}
}
