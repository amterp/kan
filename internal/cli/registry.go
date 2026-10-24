package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/registry"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/util"
	"github.com/amterp/ra"
)

func registerRegistry(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("registry")
	cmd.SetDescription("List and clean up the projects kan knows on this machine")

	listCmd := ra.NewCmd("list")
	listCmd.SetDescription("List registered projects, their boards and whether they are still there")
	ctx.RegistryListHidden, _ = ra.NewBool("hidden").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Include hidden entries").
		Register(listCmd)
	ctx.RegistryListUsed, _ = cmd.RegisterCmd(listCmd)

	hideCmd := ra.NewCmd("hide")
	hideCmd.SetDescription("Stop offering a project outside its own directory (it still works inside it)")
	ctx.RegistryHidePath, _ = ra.NewString("path").
		SetUsage("Path of the registered project").
		SetCompletionFunc(completeVisibleRegistryPaths).
		Register(hideCmd)
	ctx.RegistryHideUsed, _ = cmd.RegisterCmd(hideCmd)

	unhideCmd := ra.NewCmd("unhide")
	unhideCmd.SetDescription("Offer a hidden project again")
	ctx.RegistryUnhidePath, _ = ra.NewString("path").
		SetUsage("Path of the registered project").
		SetCompletionFunc(completeHiddenRegistryPaths).
		Register(unhideCmd)
	ctx.RegistryUnhideUsed, _ = cmd.RegisterCmd(unhideCmd)

	removeCmd := ra.NewCmd("remove")
	removeCmd.SetDescription("Delete a project's entry and its settings, such as data_location")
	ctx.RegistryRemovePath, _ = ra.NewString("path").
		SetUsage("Path of the registered project").
		SetCompletionFunc(completeAllRegistryPaths).
		Register(removeCmd)
	ctx.RegistryRemoveUsed, _ = cmd.RegisterCmd(removeCmd)

	pruneCmd := ra.NewCmd("prune")
	pruneCmd.SetDescription("Remove entries whose project folder or board folder is gone")
	ctx.RegistryPruneDryRun, _ = ra.NewBool("dry-run").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Show what would be removed without changing anything").
		Register(pruneCmd)
	ctx.RegistryPruneUsed, _ = cmd.RegisterCmd(pruneCmd)

	ctx.RegistryUsed, _ = parent.RegisterCmd(cmd)
}

// The registry commands act on the global config only, so like `kan global
// show` they work from anywhere and never open, register or migrate the
// project in the working directory.

func runRegistryList(includeHidden, jsonOutput bool) {
	cfg, err := loadGlobalConfig()
	if err != nil {
		Fatal(err)
	}
	statuses := registry.Inspect(cfg, includeHidden)

	if jsonOutput {
		if statuses == nil {
			statuses = []registry.Status{}
		}
		if err := printJson(statuses); err != nil {
			Fatal(err)
		}
		return
	}

	if len(statuses) == 0 {
		PrintInfo("No registered projects")
		return
	}
	for i, s := range statuses {
		if i > 0 {
			fmt.Println()
		}
		printRegistryStatus(s)
	}
}

func printRegistryStatus(s registry.Status) {
	header := fmt.Sprintf("%s  %s", RenderBold(util.PrettyPath(s.Path)), s.Name)
	if s.Hidden {
		header += "  " + RenderMuted("(hidden)")
	}
	fmt.Println(header)

	state := string(s.State)
	switch s.State {
	case registry.StateUnreadable:
		state += ": " + s.Error
	case registry.StateDuplicate:
		ids := make([]string, 0, len(s.Duplicates))
		for id := range s.Duplicates {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		var parts []string
		for _, id := range ids {
			others := make([]string, len(s.Duplicates[id]))
			for i, p := range s.Duplicates[id] {
				others[i] = util.PrettyPath(p)
			}
			parts = append(parts, fmt.Sprintf("%s also at %s", id, strings.Join(others, ", ")))
		}
		state += ": " + strings.Join(parts, "; ")
	}
	if s.State == registry.StateOK {
		fmt.Printf("  %s\n", RenderMuted(state))
	} else {
		fmt.Printf("  %s\n", state)
	}

	for _, b := range s.Boards {
		fmt.Printf("  %s %s %s\n", RenderMuted("•"), b.Name, RenderMuted(b.ID))
	}
}

func runRegistrySetHidden(path string, hidden bool) {
	cfg, err := loadGlobalConfig()
	if err != nil {
		Fatal(err)
	}
	key, err := registry.SetHidden(cfg, path, hidden)
	if err != nil {
		Fatal(err)
	}
	if err := store.NewGlobalStore().Save(cfg); err != nil {
		Fatal(err)
	}

	if hidden {
		PrintSuccess("Hid %s", util.PrettyPath(key))
		return
	}
	PrintSuccess("Unhid %s", util.PrettyPath(key))
	warnSharedBoards(cfg, key)
}

// warnSharedBoards warns when another visible entry holds one of the
// project's board IDs, since references to that board are now ambiguous.
func warnSharedBoards(cfg *model.GlobalConfig, key string) {
	shared := registry.SharedBoards(cfg, key)
	ids := make([]string, 0, len(shared))
	for id := range shared {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		others := make([]string, len(shared[id]))
		for i, p := range shared[id] {
			others[i] = util.PrettyPath(p)
		}
		PrintWarning("board %s is also registered at %s, so references to it are now ambiguous. "+
			"Hide one copy with 'kan registry hide <path>'.", id, strings.Join(others, ", "))
	}
}

func runRegistryRemove(path string) {
	cfg, err := loadGlobalConfig()
	if err != nil {
		Fatal(err)
	}
	key, err := registry.Remove(cfg, path)
	if err != nil {
		Fatal(err)
	}
	if err := store.NewGlobalStore().Save(cfg); err != nil {
		Fatal(err)
	}
	PrintSuccess("Removed %s and its settings from the registry", util.PrettyPath(key))
}

func runRegistryPrune(dryRun bool) {
	cfg, err := loadGlobalConfig()
	if err != nil {
		Fatal(err)
	}
	removed := registry.Prune(cfg, dryRun)
	if len(removed) == 0 {
		PrintInfo("Nothing to prune")
		return
	}

	if !dryRun {
		if err := store.NewGlobalStore().Save(cfg); err != nil {
			Fatal(err)
		}
	}
	verb := "Removed"
	if dryRun {
		verb = "Would remove"
	}
	for _, path := range removed {
		fmt.Printf("%s %s %s\n", verb, util.PrettyPath(path), RenderMuted("(missing)"))
	}
}

func completeRegistryPaths(toComplete string, include func(registry.Entry) bool) ([]string, ra.CompletionDirective) {
	cfg, err := store.NewGlobalStore().Load()
	if err != nil {
		return nil, ra.CompletionDirectiveNoFileComp
	}
	var result []string
	for _, e := range registry.Entries(cfg) {
		if include(e) && strings.HasPrefix(e.Path, toComplete) {
			result = append(result, e.Path)
		}
	}
	return result, ra.CompletionDirectiveNoFileComp
}

func completeVisibleRegistryPaths(toComplete string) ([]string, ra.CompletionDirective) {
	return completeRegistryPaths(toComplete, func(e registry.Entry) bool { return !e.Hidden })
}

func completeHiddenRegistryPaths(toComplete string) ([]string, ra.CompletionDirective) {
	return completeRegistryPaths(toComplete, func(e registry.Entry) bool { return e.Hidden })
}

func completeAllRegistryPaths(toComplete string) ([]string, ra.CompletionDirective) {
	return completeRegistryPaths(toComplete, func(registry.Entry) bool { return true })
}
