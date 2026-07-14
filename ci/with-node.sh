#!/usr/bin/env bash
#
# Run a command with the Node version pinned in .nvmrc.
#
# The built frontend bundle is committed, and CI rebuilds it on the pinned version and
# fails if the result differs. So a build on the wrong Node major doesn't fail where the
# mistake is made - it fails in CI, after the commit, as an opaque "embedded assets are
# out of date". This activates the pinned version so that simply doesn't come up.
#
# Activating beats warning: the shell's default node is routinely not the pinned one, and
# a check that only complains would make every build a two-step dance. We only refuse
# when no version manager can give us the right Node, since building anyway would produce
# a bundle CI rejects.
#
# Set KAN_SKIP_NODE_CHECK=1 to run with whatever Node is active.

set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
want=$(tr -d ' \t\n\r' < "$repo_root/.nvmrc")
want_major=${want%%.*}

active_major() {
	local version
	command -v node >/dev/null 2>&1 || return 1
	version=$(node -v 2>/dev/null) || return 1
	version=${version#v}
	printf '%s' "${version%%.*}"
}

matches() {
	[[ "$(active_major 2>/dev/null || true)" == "$want_major" ]]
}

if [[ -n "${KAN_SKIP_NODE_CHECK:-}" ]] || matches; then
	exec "$@"
fi

# nvm is a shell function, not a binary, so it has to be sourced - which is also why
# piping it silently loses the PATH change (the pipe runs it in a subshell).
nvm_dir=${NVM_DIR:-$HOME/.nvm}
if [[ -s "$nvm_dir/nvm.sh" ]]; then
	# shellcheck disable=SC1091
	. "$nvm_dir/nvm.sh" >/dev/null 2>&1 || true
	nvm use "$want" >/dev/null 2>&1 || nvm install "$want" >/dev/null 2>&1 || true
	if matches; then
		exec "$@"
	fi
fi

if command -v fnm >/dev/null 2>&1; then
	eval "$(fnm env 2>/dev/null)" >/dev/null 2>&1 || true
	fnm use "$want" >/dev/null 2>&1 || fnm install "$want" >/dev/null 2>&1 || true
	if matches; then
		exec "$@"
	fi
fi

active=$(active_major 2>/dev/null || echo "none")
cat >&2 <<EOF
Could not activate Node $want (.nvmrc); active version is $active.

The frontend bundle is committed and CI rebuilds it on Node $want, so building on a
different major version can produce a bundle CI then rejects. Install the pinned version
with your version manager, e.g.:

  nvm install $want     # or: fnm install $want

To build with the active Node anyway: KAN_SKIP_NODE_CHECK=1 make build
EOF
exit 1
