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

var removeTestCases = []struct {
	before, after []string
	remove        string
}{
	{
		before: []string{"foo", "bar", "baz"},
		remove: "foo",
		after:  []string{"bar", "baz"},
	}, {
		before: []string{"foo", "bar", "baz"},
		remove: "bar",
		after:  []string{"foo", "baz"},
	}, {
		before: []string{"foo", "bar", "baz"},
		remove: "baz",
		after:  []string{"foo", "bar"},
	}, {
		before: []string{"foo"},
		remove: "foo",
		after:  nil,
	}, {
		before: []string{"foo", "bar"},
		remove: "baz",
		after:  []string{"foo", "bar"},
	}, {
		before: []string{},
		remove: "foo",
		after:  nil,
	}, {
		before: nil,
		remove: "foo",
		after:  nil,
	},
}

// TestRemove tests removing an element from a string slice.
func TestRemove(t *testing.T) {
	for _, testCase := range removeTestCases {
		a := make([]string, len(testCase.before))
		copy(a, testCase.before)
		a = remove(a, testCase.remove)

		if !reflect.DeepEqual(a, testCase.after) {
			t.Errorf("expected %q, got %q after removing %q", testCase.after, a, testCase.remove)
		}
	}
}

var finalStateTestCases = []struct {
	key       string
	states    []uint64
	expectSum uint64
	expectStr string
}{
	{
		key:       "foo",
		states:    []uint64{1, 2, 3},
		expectSum: 1 + 2 + 3,
		expectStr: "0x1 + 0x2 + 0x3",
	}, {
		key:       "bar",
		states:    []uint64{4, 5, 6},
		expectSum: 4 + 5 + 6,
		expectStr: "0x4 + 0x5 + 0x6",
	}, {
		key:       "baz",
		states:    []uint64{},
		expectSum: 0,
		expectStr: "0",
	}, {
		key:       "bat",
		states:    []uint64{0, 0, 0},
		expectSum: 0,
		expectStr: "0",
	},
}

// TestFinal tests the calculation of final state.
func TestFinal(t *testing.T) {
	// Fully populate state.final before testing, to make sure states
	// don't get mixed up.
	keys := make([]string, len(finalStateTestCases))
	for _, testCase := range finalStateTestCases {
		keys = append(keys, testCase.key)
	}
	state := newStateMachine(keys)
	for _, testCase := range finalStateTestCases {
		state.final[testCase.key] = testCase.states
	}

	for _, testCase := range finalStateTestCases {
		if sum := state.finalState(testCase.key); sum != testCase.expectSum {
			t.Errorf("expected %d, got %d for %q final state", testCase.expectSum, sum, testCase.key)
		}
		if str := state.finalString(testCase.key); str != testCase.expectStr {
			t.Errorf("expected %q, got %q for %q final state", testCase.expectStr, str, testCase.key)
		}
	}
}

// TestDelete tests removing a key from the state machine.
func TestDelete(t *testing.T) {
	state := newStateMachine([]string{"a", "abc"})
	state.noMore = []map[rune][]string{
		map[rune][]string{'a': []string{"a"}},
		nil,
		map[rune][]string{'c': []string{"abc"}},
	}

	state.deleteKey("a")

	if _, exists := state.final["a"]; exists {
		t.Error("failed to delete key from stateMachine.final")
	}
	if len(state.noMore[0]['a']) != 0 {
		t.Error("failed to delete key from stateMachine.noMore")
	}
}
