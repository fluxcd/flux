package update

import (
	"io"
)

// PrintResults outputs a result set to the `io.Writer` provided, at
// the given level of verbosity:
//  - 2 = include skipped and ignored resources
//  - 1 = include skipped resources, exclude ignored resources
//  - 0 = exclude skipped and ignored resources
func PrintResults(out io.Writer, results Result, verbosity int) {
	NewMenu(out, results, verbosity).Print()
}
