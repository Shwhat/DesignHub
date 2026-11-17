package repo

import "testing"

// TestRepackKeepsRepositoryUsable repeats the nightly repack between edits.
// Before the fix, the second repack left the running server looking for a
// deleted pack file, and every later commit failed with "packfile not found".
func TestRepackKeepsRepositoryUsable(t *testing.T) {
	r, err := Open(t.TempDir(), who)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"a", "b", "c", "d"}
	for i, n := range names {
		if _, err := r.Commit(Main, []Change{{Path: "items/" + n + ".md", Content: []byte(n + "\n"), Expect: "new"}}, who, "add "+n); err != nil {
			t.Fatalf("commit %s after %d repacks: %v", n, i, err)
		}
		if err := r.Repack(); err != nil {
			t.Fatalf("repack %d: %v", i+1, err)
		}
	}
	for _, n := range names {
		b, _, err := r.Read(Main, "items/"+n+".md")
		if err != nil {
			t.Fatalf("read %s: %v", n, err)
		}
		if string(b) != n+"\n" {
			t.Fatalf("read %s: got %q", n, b)
		}
	}
}
