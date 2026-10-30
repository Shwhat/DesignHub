package repo

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

var who = Author{Name: "Test User", Email: "test@designhub.local"}

func TestCommitBranchMergeTag(t *testing.T) {
	dir := t.TempDir()
	r, err := Open(dir, who)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit(Main, []Change{{Path: "items/a.md", Content: []byte("a1\n"), Expect: "new"}}, who, "add a"); err != nil {
		t.Fatal(err)
	}
	b, h, err := r.Read(Main, "items/a.md")
	if err != nil || string(b) != "a1\n" {
		t.Fatalf("read: %q %v", b, err)
	}
	// stale save is refused, first save kept
	if _, err := r.Commit(Main, []Change{{Path: "items/a.md", Content: []byte("a2\n"), Expect: h}}, who, "edit a"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit(Main, []Change{{Path: "items/a.md", Content: []byte("a3\n"), Expect: h}}, who, "stale"); !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	if err := r.CreateBranch("co/CO-0001", Main); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit("co/CO-0001", []Change{{Path: "items/a.md", Content: []byte("a4\n")}, {Path: "items/b.md", Content: []byte("b\n")}}, who, "co edit"); err != nil {
		t.Fatal(err)
	}
	if b, _, _ := r.Read(Main, "items/a.md"); string(b) != "a2\n" {
		t.Fatalf("main changed by branch edit: %q", b)
	}
	m, err := r.Merge(Main, "co/CO-0001", []string{"items/a.md", "items/b.md"}, who, "release CO-0001")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Tag("baseline/CO-0001", m, who, "Baseline CO-0001", nil); err != nil {
		t.Fatal(err)
	}
	if b, _, _ := r.Read("baseline/CO-0001", "items/b.md"); string(b) != "b\n" {
		t.Fatalf("tag read: %q", b)
	}
	hist, err := r.History(Main, "items/a.md", 0)
	if err != nil || len(hist) != 3 {
		t.Fatalf("history: %d %v", len(hist), err)
	}
	if _, _, err := r.Read(Main, "nope.md"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
	if _, err := exec.LookPath("git"); err == nil {
		for _, args := range [][]string{{"fsck", "--strict"}, {"log", "--oneline", "main"}, {"tag", "-l"}} {
			out, err := exec.Command("git", append([]string{"--git-dir", dir}, args...)...).CombinedOutput()
			if err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
			if args[0] == "tag" && !strings.Contains(string(out), "baseline/CO-0001") {
				t.Fatalf("tag missing: %s", out)
			}
		}
	}
}
