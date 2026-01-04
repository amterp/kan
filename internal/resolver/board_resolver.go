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
	repoPath    string
}

// NewBoardResolver creates a new board resolver.
func NewBoardResolver(
	boardStore store.BoardStore,
	globalStore store.GlobalStore,
	prompter prompt.Prompter,
	repoPath string,
) *BoardResolver {
	return &BoardResolver{
		boardStore:  boardStore,
		globalStore: globalStore,
		prompter:    prompter,
		repoPath:    repoPath,
	}
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

	// 2. Get all boards
	boards, err := r.boardStore.List()
	if err != nil {
		return "", err
	}

	if len(boards) == 0 {
		return "", fmt.Errorf("no boards found; run 'kan init' first")
	}

	// 3. Single board - use it
	if len(boards) == 1 {
		return boards[0], nil
	}

	// 4. Check for configured default
	globalCfg, _ := r.globalStore.Load()
	if globalCfg != nil {
		if repoCfg := globalCfg.GetRepoConfig(r.repoPath); repoCfg != nil {
			if repoCfg.DefaultBoard != "" && r.boardStore.Exists(repoCfg.DefaultBoard) {
				return repoCfg.DefaultBoard, nil
			}
		}
	}

	// 5. Multiple boards, no default
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
