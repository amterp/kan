package git

import (
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// Remotes returns the repo's remotes at dir, by name, with their URLs. A
// directory that isn't a repo, or a repo without remotes, has none.
func (c *Client) Remotes(dir string) map[string]string {
	if !c.IsRepoAt(dir) {
		return nil
	}
	cmd := exec.Command("git", "config", "--get-regexp", `^remote\..*\.url$`)
	cmd.Dir = dir
	// A GIT_DIR inherited from a hook would point at the repo kan runs in,
	// not the one at dir.
	cmd.Env = withoutRepoOverrides(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	remotes := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		key, value, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(key, "remote."), ".url")
		remotes[name] = value
	}
	return remotes
}

// FetchURL picks the remote someone could fetch the repo from: origin, or
// the only remote. With several remotes and none named origin, or with none,
// there is no single answer and it returns "".
func FetchURL(remotes map[string]string) string {
	if u, ok := remotes["origin"]; ok {
		return u
	}
	if len(remotes) == 1 {
		for _, u := range remotes {
			return u
		}
	}
	return ""
}

// StripCredentials removes secrets from a remote URL, so it can be written
// to a committed file. A URL with a scheme loses its user info, except that
// ssh:// keeps the user name, which is a login, not a secret. The scp-like
// form (git@github.com:example/api.git) has no password and is kept as is.
func StripCredentials(remote string) string {
	if !strings.Contains(remote, "://") {
		return remote
	}
	u, err := url.Parse(remote)
	if err != nil || u.User == nil {
		return remote
	}
	if u.Scheme == "ssh" {
		u.User = url.User(u.User.Username())
	} else {
		u.User = nil
	}
	return u.String()
}

func withoutRepoOverrides(env []string) []string {
	kept := env[:0:0]
	for _, kv := range env {
		if strings.HasPrefix(kv, "GIT_DIR=") || strings.HasPrefix(kv, "GIT_WORK_TREE=") {
			continue
		}
		kept = append(kept, kv)
	}
	return kept
}
