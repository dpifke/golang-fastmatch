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
	"io"
	"sort"
	"strconv"
)

// ErrBadFlags is returned when nonsensical flags are passed to Generate.
type ErrBadFlags struct {
	cannotCombine    []string
	cannotStopIgnore sortableRunes
}

// writeListSeparator outputs a list separator between items in a list.
//
// It uses an Oxford comma, because:
// http://i3.kym-cdn.com/photos/images/newsfeed/000/946/427/5a4.jpg
func writeListSeparator(w io.Writer, n, last int) {
	if n == last {
		if n == 1 {
			// last of two-item list
			w.Write([]byte(" and "))
		} else {
			// last of longer list
			w.Write([]byte(", and "))
		}
	} else {
		w.Write([]byte(", "))
	}
}

func (e *ErrBadFlags) Error() string {
	b := new(bytes.Buffer)

	sort.Strings(e.cannotCombine)
	for n, key := range e.cannotCombine {
		if n == 0 {
			b.WriteString("flags are mutually exclusive: ")
		} else {
			writeListSeparator(b, n, len(e.cannotCombine)-1)
		}
		b.WriteString(strconv.Quote(key))
	}

	sort.Sort(e.cannotStopIgnore)
	for n, r := range e.cannotStopIgnore {
		if n == 0 {
			if b.Len() != 0 {
				b.WriteString("; ")
			}
			b.WriteString("runes in StopUpon cannot be equivalent to runes in Ignore: ")
		} else {
			writeListSeparator(b, n, len(e.cannotStopIgnore)-1)
		}
		b.WriteString(strconv.QuoteRune(r))
	}

	return b.String()
}

// Flag can be passed to Generate and GenerateReverse to modify the functions'
// behavior.  Users of this package should not instantiate their own Flags.
// Rather, they should use one of HasPrefix, HasSuffix, Insensitive,
// Normalize, or the return value from Equivalent(), StopUpon(), Ignore(), or
// IgnoreExcept().  Unknown Flags are silently discarded.
type Flag struct {
	equivalent, stop, ignore, ignoreExcept []rune
}

// Insensitive is a flag, which can be passed to Generate, to specify that
// matching should be case-insensitive.
var Insensitive = new(Flag)

// Normalize is a flag, which can be passed to Generate, to specify that
// matching should be done without regard to diacritics, accents, etc.
//
// This is currently just a placeholder, and has no effect yet on the
// generated code.
var Normalize = new(Flag)

// Equivalent is a flag, which can be passed to Generate, to specify
// runes that should be treated identically when matching.
func Equivalent(runes ...rune) *Flag {
	return &Flag{equivalent: runes}
}

// HasPrefix is a flag, which can be passed to Generate, to specify that
// runes proceeding a match should be ignored.
//
// Matching stops as soon as a match is found, thus "foo" and "f" are
// ambiguous cases when HasPrefix is specified.  Generate returns an error if
// ambiguity is detected.
var HasPrefix = new(Flag)

// HasSuffix is a flag, which can be passed to Generate, to match the end of
// the input string, in the same manner HasPrefix performs a match of the
// beginning of the string.
var HasSuffix = new(Flag)

// StopUpon is a flag, which can be passed to Generate, to specify a set of
// runes (including equivalents) which get treated like a string boundary,
// i.e. cause matching to immediately cease.
//
// In normal usage, this results in matches which must be immediately followed
// by either end-of-string or the stop character.  This is similar to
// HasPrefix, except that it allows for matches that would be ambiguous if we
// stopped upon first match.  Consider a matcher for URI schemes (RFC 7595),
// where we want to match either the bare scheme name or the beginning of a
// URL.  We might generate a matcher as follows:
//
//	fmt.Fprintln(w, "func matchScheme(input string) Scheme {")
//	fastmatch.Generate(w, map[string]string{
//		"http": "HTTP",
//		"https": "HTTPS",
//	}, "nil", fastmatch.Insensitive, fastmatch.StopUpon(':'))
//
// With the above, matchScheme("http") and matchScheme("http://example.com")
// both return HTTP, and matchScheme("https") and
// matchScheme("HTTPS://example.com") both return HTTPS.
// matchScheme("https+xml://example.com") would return nil.
//
// When StopUpon is combined with HasSuffix, the stop character is treated as
// the beginning of the string.  (This is obvious if one considers we match
// from right-to-left when HasSuffix is specified.)  An example filename
// extension matcher could be generated as follows:
//
//	fmt.Fprintln(w, "func matchExt(input string) Extension {")
//	fastmatch.Generate(w, map[string]string{
//		"exe": "EXE",
//		"dll": "DLL",
//	}, "nil", fastmatch.StopUpon('.'), fastmatch.HasSuffix)
//
// matchExt("foo.exe") and matchExt("exe") both return EXE, and
// matchExt("bar.dll") and matchExt("dll") both return DLL.
//
// Runes from StopUpon may not also appear in Ignore.  If IgnoreExcept is
// specified, runes from StopUpon will be treated as stop runes regardless of
// whether or not they appear in IgnoreExcept.
func StopUpon(runes ...rune) *Flag {
	return &Flag{stop: runes}
}

// Ignore is a flag, which can be passed to Generate, to specify runes
// (including equivalents) which should be ignored for matching purposes.
func Ignore(runes ...rune) *Flag {
	return &Flag{ignore: runes}
}

// IgnoreExcept is a flag, which can be passed to Generate, to specify which
// runes (including equivalents) should be examined when matching.  This is
// similar to Ignore, except that when this flag is present, any runes not
// listed will be ignored.
//
// Ignore and IgnoreExcept may not be combined.
func IgnoreExcept(runes ...rune) *Flag {
	return &Flag{ignoreExcept: runes}
}

// Range accepts zero or more pairs of runes, and returns a slice covering all
// runes between the even and odd arguments, inclusive.  It can be used with
// flags which take a list of runes as arguments, such as Equivalent,
// StopUpon, Ignore, or IgnoreExcept.
//
// For example:
//
//	fastmatch.Generate(w, cases, "nil",
//		fastmatch.IgnoreExcept(Range('0', '9', 'a', 'z', 'A', 'Z')...))
//
// If passed an odd number of arguments, this function will panic.
//
// See also the predefined ranges: Numbers, Lowercase, Uppercase, Letters,
// and Alphanumeric.
func Range(args ...rune) []rune {
	if len(args)%2 != 0 {
		panic("wrong number of arguments to Range")
	}

	l := 0
	for i := 0; i < len(args); i += 2 {
		l += int(args[i+1]-args[i]) + 1
	}
	rs := make([]rune, 0, l)

	for i := 0; i < len(args); i += 2 {
		for j := 0; j <= int(args[i+1]-args[i]); j++ {
			rs = append(rs, args[i]+rune(j))
		}
	}

	return rs
}

// Numbers is a predefined Range covering the ASCII digits from 0 through 9.
var Numbers = Range('0', '9')

// Letters is a predefined Range covering upper- and lower-case ASCII letters.
var Letters = Range('a', 'z', 'A', 'Z')

// Lowercase is a predefined Range covering lower-case ASCII letters.
var Lowercase = Range('a', 'z')

// Uppercase is a predefined Range covering upper-case ASCII letters.
var Uppercase = Range('A', 'Z')

// Alphanumeric is a predefined Range covering ASCII numeric digits and
// upper- and lower-case letters.
var Alphanumeric = Range('0', '9', 'a', 'z', 'A', 'Z')
