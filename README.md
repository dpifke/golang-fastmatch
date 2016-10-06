# pifke.org/fastmatch

[![GoDoc](https://godoc.org/pifke.org/fastmatch?status.svg)](https://godoc.org/pifke.org/fastmatch)
[![Test Coverage](https://coveralls.io/repos/github/dpifke/golang-fastmatch/badge.svg)](https://coveralls.io/github/dpifke/golang-fastmatch)
[![Build Status](https://drone.io/github.com/dpifke/golang-fastmatch/status.png)](https://drone.io/github.com/dpifke/golang-fastmatch/latest)

Golang code generation tool for quickly comparing an input string to a set of
possible matches, which are known at compile time.

A typical use of this would be generating a "reverse enum", such as in a
parser which needs to compare a string to a list of keywords and return the
corresponding lexer symbol.

For more information, see the
[documentation](https://godoc.org/pifke.org/fastmatch).

## Downloading

If you use this library in your own code, please use the canonical URL in your
Go code, instead of Github:

```
go get pifke.org/fastmatch
```

Or (until I finish setting up the self-hosted repository):

```
# From the root of your project:
git submodule add https://github.com/dpifke/golang-fastmatch vendor/pifke.org/fastmatch
```

Then:

```
import (
        "pifke.org/fastmatch"
)
```

As opposed to the pifke.org URL, I make no guarantee this Github repository
will exist or be up-to-date in the future.

## Documentation

Available on [godoc.org](https://godoc.org/pifke.org/fastmatch).

## License

Three-clause BSD.  See LICENSE.txt.

Contact me if you want to use this code under different terms.

## Author

Dave Pifke.  My email address is my first name "at" my last name "dot org."

I'm [@dpifke](https://twitter.com/dpifke) on Twitter.  My PGP key
is available on [Keybase](https://keybase.io/dpifke).
