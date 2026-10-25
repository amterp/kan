package git

import (
	"os/exec"
	"testing"
)

func TestFetchURL(t *testing.T) {
	cases := []struct {
		name    string
		remotes map[string]string
		want    string
	}{
		{"origin wins", map[string]string{"origin": "o", "fork": "f"}, "o"},
		{"only remote", map[string]string{"upstream": "u"}, "u"},
		{"several without origin", map[string]string{"a": "1", "b": "2"}, ""},
		{"none", nil, ""},
	}
	for _, c := range cases {
		if got := FetchURL(c.remotes); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestShareableRemote(t *testing.T) {
	cases := map[string]string{
		"https://user:token@github.com/example/api.git":   "https://github.com/example/api.git",
		"https://token@github.com/example/api.git":        "https://github.com/example/api.git",
		"https://github.com/example/api.git":              "https://github.com/example/api.git",
		"https://github.com/example/api.git?token=abc#x":  "https://github.com/example/api.git",
		"ssh://git:secret@github.com/example/api.git":     "ssh://git@github.com/example/api.git",
		"ssh://git@github.com/example/api.git":            "ssh://git@github.com/example/api.git",
		"git+ssh://git:secret@github.com/example/api.git": "git+ssh://git@github.com/example/api.git",
		"git@github.com:example/api.git":                  "git@github.com:example/api.git",
		"github.com:example/api.git":                      "github.com:example/api.git",
		"https://user:pa%ssword@github.com/a/b.git":       "",
		"https://user:pa/ss@github.com/a/b.git":           "",
		"https://user:pa#ss@github.com/a/b.git":           "",
		"https://user:pa?ss@github.com/a/b.git":           "",
		"https://user:pa ss@github.com/a/b.git":           "",
		"user:secret@github.com:example/api.git":          "",
		"file:///srv/git/api.git":                         "",
		"/srv/git/api.git":                                "",
		"../api.git":                                      "",
		`C:\repos\api.git`:                                "",
		"":                                                "",
	}
	for in, want := range cases {
		if got := ShareableRemote(in); got != want {
			t.Errorf("ShareableRemote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRemotes(t *testing.T) {
	dir := initTestRepo(t)
	if got := NewClient().Remotes(dir); len(got) != 0 {
		t.Errorf("a repo without remotes: got %v", got)
	}

	for name, u := range map[string]string{"origin": "https://example.com/api.git", "fork": "git@example.com:me/api.git"} {
		if out, err := exec.Command("git", "-C", dir, "remote", "add", name, u).CombinedOutput(); err != nil {
			t.Fatalf("git remote add: %v: %s", err, out)
		}
	}
	got := NewClient().Remotes(dir)
	if got["origin"] != "https://example.com/api.git" || got["fork"] != "git@example.com:me/api.git" || len(got) != 2 {
		t.Errorf("Remotes = %v", got)
	}

	if got := NewClient().Remotes(t.TempDir()); got != nil {
		t.Errorf("not a repo: got %v", got)
	}
}
