package resolver

import (
	"fmt"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/prompt"
	"github.com/amterp/kan/internal/store"
)

// BoardResolver handles board selection logic.
type BoardResolver struct {
	boardStore  store.BoardStore
	globalStore store.GlobalStore
	prompter    prompt.Prompter
	projectPath string
	// preferredBoard, when set, is used ahead of default_board and the
	// interactive picker. It is how `-g` pins resolution to the designated
	// global board while still letting an explicit -b override.
	preferredBoard string
}

// NewBoardResolver creates a new board resolver.
func NewBoardResolver(
	boardStore store.BoardStore,
	globalStore store.GlobalStore,
	prompter prompt.Prompter,
	projectPath string,
) *BoardResolver {
	return &BoardResolver{
		boardStore:  boardStore,
		globalStore: globalStore,
		prompter:    prompter,
		projectPath: projectPath,
	}
}

// SetPreferredBoard pins resolution to a specific board (when no explicit board
// is given), used by `-g` to target the designated global board.
func (r *BoardResolver) SetPreferredBoard(board string) {
	r.preferredBoard = board
}

// InferBoard resolves which board to use without user interaction.
// It checks: single-board auto-detect, then default_board from global config.
// Returns "" if no board can be inferred. Used by both BoardResolver and
// shell completion (which runs before the full App is available).
func InferBoard(boardStore store.BoardStore, globalCfg *model.GlobalConfig, projectRoot string) string {
	boards, err := boardStore.List()
	if err != nil || len(boards) == 0 {
		return ""
	}

	if len(boards) == 1 {
		return boards[0]
	}

	if globalCfg != nil && projectRoot != "" {
		if repoCfg := globalCfg.GetRepoConfig(projectRoot); repoCfg != nil {
			if repoCfg.DefaultBoard != "" {
				return repoCfg.DefaultBoard
			}
		}
	}

	return ""
}

// Resolve determines which board to use based on the spec's rules:
// 1. If explicit board provided, use it
// 2. If only one board exists, use it
// 3. If default_board configured, use it
// 4. If interactive, prompt user
// 5. Otherwise, fail with error
func (r *BoardResolver) Resolve(explicitBoard string, interactive bool) (string, error) {
	// 1. Explicit board
	if explicitBoard != "" {
		if !r.boardStore.Exists(explicitBoard) {
			return "", fmt.Errorf("board %q not found", explicitBoard)
		}
		return explicitBoard, nil
	}

	// 2. Get all boards - check for empty first
	boards, err := r.boardStore.List()
	if err != nil {
		return "", err
	}
	if len(boards) == 0 {
		return "", fmt.Errorf("no boards found; run 'kan init' first")
	}

	// 2.5. Preferred board (e.g. the designated global board under -g) wins over
	// default_board and the picker, but yields to an explicit -b (handled above).
	if r.preferredBoard != "" && r.boardStore.Exists(r.preferredBoard) {
		return r.preferredBoard, nil
	}

	// 3-4. Try non-interactive inference (single board or default)
	globalCfg, _ := r.globalStore.Load()
	if board := InferBoard(r.boardStore, globalCfg, r.projectPath); board != "" {
		// Validate the inferred board still exists (default_board might be stale)
		if r.boardStore.Exists(board) {
			return board, nil
		}
	}

	// 5. Multiple boards, no usable default
	if !interactive {
		return "", fmt.Errorf("multiple boards exist; specify with -b or set default_board in config")
	}

	// 6. Prompt user
	return r.prompter.Select("Select board", boards)
}

// GetBoardConfig returns the board configuration.
func (r *BoardResolver) GetBoardConfig(boardName string) (*model.BoardConfig, error) {
	return r.boardStore.Get(boardName)
}
