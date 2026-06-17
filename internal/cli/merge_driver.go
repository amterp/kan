package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	gd "github.com/amterp/go-delta"
	"github.com/amterp/kan/internal/merge"
)

// runMergeDriver implements the git merge driver for Kan card files. Git invokes
// it (via the merge.kan.driver config installed by gitdriver) with a fixed
// argument list derived from the placeholders in that config:
//
//	kan merge-driver %O %A %B %P %L
//	                 base ours theirs path marker-size
//
// %A is both the "ours" input and the output file: the driver writes the merged
// result there. Exit 0 means a clean merge; a non-zero exit tells git the file
// is still conflicted (the bytes we wrote then contain conflict markers).
//
// This path is intentionally not a registered subcommand - it's an integration
// point for git, not a user-facing command, so it's dispatched directly from
// Run() and kept out of help and completion.
func runMergeDriver(args []string) {
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "kan merge-driver: expected at least %O %A %B (base ours theirs)")
		os.Exit(2)
	}
	basePath, oursPath, theirsPath := args[0], args[1], args[2]

	pathName := oursPath
	if len(args) >= 4 && args[3] != "" {
		pathName = args[3]
	}
	markerLen := 7
	if len(args) >= 5 {
		if n, err := strconv.Atoi(args[4]); err == nil && n > 0 {
			markerLen = n
		}
	}

	// base may legitimately be empty/absent (an add/add conflict); only ours and
	// theirs must exist.
	base, _ := os.ReadFile(basePath)
	ours, err := os.ReadFile(oursPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kan merge-driver: read ours: %v\n", err)
		os.Exit(2)
	}
	theirs, err := os.ReadFile(theirsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kan merge-driver: read theirs: %v\n", err)
		os.Exit(2)
	}

	result, conflicts, err := merge.Cards(base, ours, theirs, markerLen)
	if err != nil {
		// We can't merge this semantically (e.g. unparseable card, delete/modify).
		// Leave ours' file untouched and report a conflict so git falls back to
		// its normal unresolved-conflict handling.
		fmt.Fprintf(os.Stderr, "kan merge-driver: %s: %v\n", pathName, err)
		os.Exit(1)
	}

	if err := os.WriteFile(oursPath, result, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "kan merge-driver: write %s: %v\n", oursPath, err)
		os.Exit(2)
	}

	if len(conflicts) > 0 {
		fmt.Fprintf(os.Stderr, "kan merge-driver: %s needs manual resolution:\n", pathName)
		for _, c := range conflicts {
			fmt.Fprintf(os.Stderr, "  %s (ours vs theirs):\n", c.Field)
			// go-delta auto-detects whether stderr is a TTY for color.
			diff := gd.Diff(c.Ours, c.Theirs)
			fmt.Fprintln(os.Stderr, "    "+strings.ReplaceAll(diff, "\n", "\n    "))
		}
		os.Exit(1)
	}
}
