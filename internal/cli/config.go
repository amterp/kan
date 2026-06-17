package cli

import (
	"fmt"
	"os"

	"github.com/amterp/kan/internal/gitdriver"
	"github.com/amterp/ra"
)

func registerConfig(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("config")
	cmd.SetDescription("View and change Kan configuration")

	mdCmd := ra.NewCmd("merge-driver")
	mdCmd.SetDescription("Turn the git merge driver (auto-resolves card conflicts) on or off")

	ctx.ConfigMergeDriverState, _ = ra.NewString("state").
		SetUsage("on or off").
		Register(mdCmd)

	ctx.ConfigMergeDriverGlobal, _ = ra.NewBool("global").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Apply to all projects (global config) instead of just this one").
		Register(mdCmd)

	ctx.ConfigMergeDriverUsed, _ = cmd.RegisterCmd(mdCmd)
	ctx.ConfigUsed, _ = parent.RegisterCmd(cmd)
}

func parseOnOff(s string) (bool, error) {
	switch s {
	case "on", "true", "enable", "enabled", "yes":
		return true, nil
	case "off", "false", "disable", "disabled", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid state %q (expected 'on' or 'off')", s)
	}
}

func onOffWord(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func runConfigMergeDriver(state string, global bool) {
	enabled, err := parseOnOff(state)
	if err != nil {
		Fatal(err)
	}

	// Skip auto-heal so it doesn't install the driver moments before we'd turn
	// it off (or vice versa).
	app, err := NewAppWithOptions(AppOptions{SkipMergeDriverAutoHeal: true})
	if err != nil {
		Fatal(err)
	}

	if global {
		gc, err := app.GlobalStore.Load()
		if err != nil {
			Fatal(err)
		}
		gc.MergeDriver = &enabled
		if err := app.GlobalStore.Save(gc); err != nil {
			Fatal(err)
		}
		PrintSuccess("Merge driver turned %s globally", onOffWord(enabled))
		if enabled {
			PrintInfo("Projects with their own \"off\" setting stay disabled.")
		} else {
			PrintInfo("Projects with their own \"on\" setting stay enabled; repos that already installed the driver keep it until you run `kan config merge-driver off` there.")
		}
		return
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}
	pc, err := app.ProjectStore.Load()
	if err != nil {
		Fatal(err)
	}
	pc.MergeDriver = &enabled
	if err := app.ProjectStore.Save(pc); err != nil {
		Fatal(err)
	}

	// Apply the change to this repo right away.
	var wroteAttrs bool
	repoRoot, kanRel, kanExe, ok, derr := driverPaths(app.GitClient, app.ProjectRoot, app.Paths.KanRoot())
	if ok && derr == nil {
		if enabled {
			res, ierr := gitdriver.Install(app.GitClient, repoRoot, kanRel, kanExe)
			if ierr != nil {
				PrintWarning("saved config, but failed to install driver: %v", ierr)
			}
			wroteAttrs = res.WroteAttributes
		} else if uerr := gitdriver.Uninstall(app.GitClient, repoRoot, kanRel); uerr != nil {
			PrintWarning("saved config, but failed to uninstall driver: %v", uerr)
		}
	}

	PrintSuccess("Merge driver turned %s for this project", onOffWord(enabled))
	switch {
	case enabled && wroteAttrs:
		fmt.Fprintln(os.Stderr, RenderMuted("  Commit .gitattributes to enable it for collaborators too."))
	case !enabled:
		fmt.Fprintln(os.Stderr, RenderMuted("  Commit the removed .gitattributes entry to disable it for collaborators too."))
	}
}
