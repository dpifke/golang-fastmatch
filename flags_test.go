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
	"io/ioutil"
	"reflect"
	"strings"
	"testing"
)

// typeOf returns the type name of a value, including pointer dereferences.
func typeOf(v interface{}) string {
	var b bytes.Buffer
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		b.WriteRune('*')
		t = t.Elem()
	}
	b.WriteString(t.Name())
	return b.String()
}

// TestBadFlags tests that Generate complains if passed impossible flags.
func TestBadFlags(t *testing.T) {
	for _, flags := range [][]*Flag{
		[]*Flag{HasPrefix, HasSuffix},
		[]*Flag{Normalize, HasSuffix, Insensitive, HasPrefix},
	} {
		err := Generate(ioutil.Discard, map[string]string{"a": "1"}, "0", flags...)
		if err == nil {
			t.Errorf("failed to trigger ErrBadFlags")
		} else if err, ok := err.(*ErrBadFlags); !ok {
			t.Errorf("expected *ErrBadFlags, got %s: %q", typeOf(err), err.Error())
		} else {
			errstr := err.Error()
			if strings.Count(errstr, "HasPrefix") != 1 || strings.Count(errstr, "HasSuffix") != 1 {
				t.Error("unexpected content from *ErrBadFlags.Error(): ", errstr)
			}
		}
	}
}
