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

/*
Package fastmatch provides a code generation tool for quickly comparing an
input string to a set of possible matches which are known at compile time.

A typical use of this would be a "reverse enum", such as in a parser which
needs to compare a string to a list of keywords and return the corresponding
lexer symbol.

Normally, the easiest way to do this would be with a switch statement, such
as:

	switch (input) {
	case "foo":
		return foo
	case "bar":
		return bar
	case "baz":
		return baz
	}

The compiled code for the above will compare the input to each string in
sequence.  If input doesn't match "foo", we try to match "bar", then "baz".
The matching process starts anew for each case.  If we have lots of possible
matches, this can be a lot of wasted effort.

Another option would be to use a map, on the (probably valid) assumption that
Go's map lookups are faster than executing a bunch of string comparisons in
sequence:

	match := map[string]int{
		"foo": foo,
		"bar": bar,
		"baz": baz,
	}
	return match[input]

The compiled code for the above will recreate the map at runtime.  We thus
have to hash each possible match every time the map is initialized, allocate
memory, garbage collect it, etc.  More wasted effort.

And this is all not to mention the potential complications related to
case-insensitive matching, partial matches (e.g. strings.HasPrefix and
strings.HasSuffix), Unicode normalization, or situations where we want to
treat a class of characters (such as all numeric digits) as equivalent for
matching purposes.  You could use a regular expression, but now you'd have two
problems, as the jwz quote goes.

The code generated by this package is theoretically more efficient than the
preceding approaches.  It supports partial matches, and can treat groups of
characters (e.g. 'a' and 'A') as equivalent.

Under the hood, it works by partitioning the search space by the length of the
input string, then updating a state machine based on each rune in the input.
If the character at a given position in the input doesn't correspond to any
possible match, we bail early.  Otherwise, the final state is compared against
possible matches using a final switch statement.

Is the code output by this package faster enough to matter?  Maybe, maybe not.
This is a straight port of a C code generation tool I've used on a couple of
projects.  In C, the difference was significant, due to strcmp() or
strcasecmp() function call overhead, and GCC's ability to convert long switch
statements into jump tables or binary searches.

Go (as of 1.7) doesn't yet do any optimization of switch statements.  See
https://github.com/golang/go/issues/5496 and
https://github.com/golang/go/issues/15780.  Thus, you may actually be worse
off in the short-term for using this method instead of a map lookup.
(Certainly in terms of code size.)  But as the compiler improves, this code
will become more relevant.  I've played with having this package output
assembler code, but it seems like the effort would be better spent improving
the compiler instead.
*/
package fastmatch
