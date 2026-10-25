package git

import (
	"net/url"
	"os"
	"os/exec"
	"regexp"
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

// ShareableRemote returns a form of a remote URL that is safe to write to
// a committed file and useful to someone on another machine, or "" when
// there is none. It fails closed: anything it can't parse is dropped, not
// kept as is.
//   - A URL with a scheme loses its user info (ssh:// and its git+ssh://
//     spellings keep the user name, a login rather than a secret), query and
//     fragment.
//   - The scp-like form (git@github.com:example/api.git) has no password
//     and is kept when its user and host are plain.
//   - A local path or file:// URL is dropped: it names a place on this
//     machine only, and would publish the user's directory layout.
func ShareableRemote(remote string) string {
	if strings.Contains(remote, "://") {
		u, err := url.Parse(remote)
		if err != nil || u.Scheme == "file" || u.Host == "" {
			return ""
		}
		if u.User != nil && (u.Scheme == "ssh" || u.Scheme == "git+ssh" || u.Scheme == "ssh+git") {
			u.User = url.User(u.User.Username())
		} else {
			u.User = nil
		}
		u.RawQuery, u.ForceQuery, u.Fragment, u.RawFragment = "", false, "", ""
		return u.String()
	}
	if isPlainSCP(remote) {
		return remote
	}
	return ""
}

var scpUserHost = regexp.MustCompile(`^([A-Za-z0-9._-]+@)?[A-Za-z0-9.-]{2,}$`)

// isPlainSCP reports whether remote is git's scp-like syntax,
// [user@]host:path, with a plain user and host and a path holding no '@',
// ':' or backslash, any of which could be a mistyped password or a Windows
// path. A Windows drive (C:\...) has a one-letter host, so it doesn't
// match either.
func isPlainSCP(remote string) bool {
	userHost, path, ok := strings.Cut(remote, ":")
	return ok && path != "" && scpUserHost.MatchString(userHost) && !strings.ContainsAny(path, "@:\\")
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
