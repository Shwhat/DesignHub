// Package repo stores engineering data and pages in a bare git repository
// using go-git. Every save is a commit. Change orders work on their own
// branches; release merges a branch into main and creates a baseline tag.
package repo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage"
)

// Main is the branch that holds released data and uncontrolled drafts.
const Main = "main"

var (
	// ErrConflict means a file changed since the caller read it.
	ErrConflict = errors.New("changed since it was opened")
	// ErrNotFound means the file, branch or tag does not exist.
	ErrNotFound = errors.New("not found")
)

// Repo is a bare git repository. Writes are serialized.
type Repo struct {
	mu  sync.Mutex
	g   *git.Repository
	dir string
}

// Author identifies the signed-in user who makes a change.
type Author struct {
	Name  string
	Email string
}

// Change is one file written or deleted in a commit.
type Change struct {
	Path    string
	Content []byte
	Delete  bool
	// Expect is the blob hash the caller read. When set, the commit fails
	// with ErrConflict if the file now has a different hash. Use "new" to
	// require that the file does not exist yet.
	Expect string
}

// Open opens the repository in dir, creating it with an initial commit on
// main if it does not exist.
func Open(dir string, a Author) (*Repo, error) {
	g, err := git.PlainOpen(dir)
	if errors.Is(err, git.ErrRepositoryNotExists) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		g, err = git.PlainInitWithOptions(dir, &git.PlainInitOptions{
			Bare:        true,
			InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName(Main)},
		})
		if err != nil {
			return nil, fmt.Errorf("create repository: %w", err)
		}
		r := &Repo{g: g, dir: dir}
		readme := []byte("# DesignHub engineering data\n\nEngineering items and pages are stored as markdown files with YAML frontmatter.\n")
		if _, err := r.commit(Main, plumbing.ZeroHash, []Change{{Path: "README.md", Content: readme}}, a, "Create engineering repository", nil); err != nil {
			return nil, err
		}
		return r, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	return &Repo{g: g, dir: dir}, nil
}

// Dir returns the repository folder.
func (r *Repo) Dir() string { return r.dir }

// Head returns the commit hash a branch or tag points to.
func (r *Repo) Head(ref string) (string, error) {
	h, err := r.resolve(ref)
	if err != nil {
		return "", err
	}
	return h.String(), nil
}

func (r *Repo) resolve(ref string) (plumbing.Hash, error) {
	if plumbing.IsHash(ref) {
		return plumbing.NewHash(ref), nil
	}
	if b, err := r.g.Reference(plumbing.NewBranchReferenceName(ref), true); err == nil {
		return b.Hash(), nil
	}
	if t, err := r.g.Reference(plumbing.NewTagReferenceName(ref), true); err == nil {
		if tag, err := r.g.TagObject(t.Hash()); err == nil {
			return tag.Target, nil
		}
		return t.Hash(), nil
	}
	return plumbing.ZeroHash, fmt.Errorf("%s: %w", ref, ErrNotFound)
}

func (r *Repo) tree(ref string) (*object.Tree, error) {
	h, err := r.resolve(ref)
	if err != nil {
		return nil, err
	}
	c, err := r.g.CommitObject(h)
	if err != nil {
		return nil, err
	}
	return c.Tree()
}

// Read returns a file's content and blob hash at a branch, tag or commit.
func (r *Repo) Read(ref, p string) ([]byte, string, error) {
	t, err := r.tree(ref)
	if err != nil {
		return nil, "", err
	}
	f, err := t.File(p)
	if err != nil {
		if errors.Is(err, object.ErrFileNotFound) || errors.Is(err, object.ErrDirectoryNotFound) || errors.Is(err, object.ErrEntryNotFound) {
			return nil, "", fmt.Errorf("%s: %w", p, ErrNotFound)
		}
		return nil, "", err
	}
	rd, err := f.Reader()
	if err != nil {
		return nil, "", err
	}
	defer rd.Close()
	b, err := io.ReadAll(rd)
	return b, f.Hash.String(), err
}

// Entry is a file in a listing.
type Entry struct {
	Path string
	Hash string
}

// List returns every file under prefix at ref.
func (r *Repo) List(ref, prefix string) ([]Entry, error) {
	t, err := r.tree(ref)
	if err != nil {
		return nil, err
	}
	var out []Entry
	err = t.Files().ForEach(func(f *object.File) error {
		if prefix == "" || strings.HasPrefix(f.Name, prefix) {
			out = append(out, Entry{f.Name, f.Hash.String()})
		}
		return nil
	})
	return out, err
}

// Commit writes changes to branch as one commit and returns its hash.
func (r *Repo) Commit(branch string, changes []Change, a Author, msg string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	parent, err := r.resolve(branch)
	if err != nil {
		return "", err
	}
	h, err := r.commit(branch, parent, changes, a, msg, nil)
	return h.String(), err
}

// commit builds a new tree from the parent's tree plus changes, writes the
// commit and moves the branch. extraParents makes a merge commit.
func (r *Repo) commit(branch string, parent plumbing.Hash, changes []Change, a Author, msg string, extraParents []plumbing.Hash) (plumbing.Hash, error) {
	files := map[string]plumbing.Hash{}
	if !parent.IsZero() {
		pc, err := r.g.CommitObject(parent)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		pt, err := pc.Tree()
		if err != nil {
			return plumbing.ZeroHash, err
		}
		if err := pt.Files().ForEach(func(f *object.File) error {
			files[f.Name] = f.Hash
			return nil
		}); err != nil {
			return plumbing.ZeroHash, err
		}
	}
	for _, c := range changes {
		p, err := cleanPath(c.Path)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		cur, exists := files[p]
		switch {
		case c.Expect == "new" && exists:
			return plumbing.ZeroHash, fmt.Errorf("%s already exists: %w", p, ErrConflict)
		case c.Expect != "" && c.Expect != "new" && (!exists || cur.String() != c.Expect):
			return plumbing.ZeroHash, fmt.Errorf("%s %w", p, ErrConflict)
		}
		if c.Delete {
			delete(files, p)
			continue
		}
		h, err := r.writeBlob(c.Content)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		files[p] = h
	}
	root, err := r.writeTree(files)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	sig := object.Signature{Name: a.Name, Email: a.Email, When: time.Now()}
	c := &object.Commit{Author: sig, Committer: sig, Message: msg, TreeHash: root}
	if !parent.IsZero() {
		c.ParentHashes = append([]plumbing.Hash{parent}, extraParents...)
	}
	obj := r.g.Storer.NewEncodedObject()
	if err := c.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}
	h, err := r.g.Storer.SetEncodedObject(obj)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	name := plumbing.NewBranchReferenceName(branch)
	var old *plumbing.Reference
	if !parent.IsZero() {
		old = plumbing.NewHashReference(name, parent)
	}
	if err := r.g.Storer.CheckAndSetReference(plumbing.NewHashReference(name, h), old); err != nil {
		if errors.Is(err, storage.ErrReferenceHasChanged) {
			return plumbing.ZeroHash, fmt.Errorf("branch %s %w", branch, ErrConflict)
		}
		return plumbing.ZeroHash, err
	}
	return h, nil
}

func cleanPath(p string) (string, error) {
	c := path.Clean(strings.TrimPrefix(p, "/"))
	if c == "." || strings.HasPrefix(c, "../") || c == ".." || strings.Contains(c, "\x00") || strings.HasPrefix(c, ".git") {
		return "", fmt.Errorf("invalid path %q", p)
	}
	return c, nil
}

func (r *Repo) writeBlob(b []byte) (plumbing.Hash, error) {
	obj := r.g.Storer.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)
	obj.SetSize(int64(len(b)))
	w, err := obj.Writer()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if _, err := w.Write(b); err != nil {
		return plumbing.ZeroHash, err
	}
	if err := w.Close(); err != nil {
		return plumbing.ZeroHash, err
	}
	return r.g.Storer.SetEncodedObject(obj)
}

// writeTree writes nested tree objects for a flat path to blob map.
func (r *Repo) writeTree(files map[string]plumbing.Hash) (plumbing.Hash, error) {
	type dir struct {
		files map[string]plumbing.Hash
		dirs  map[string]*dir
	}
	newDir := func() *dir { return &dir{map[string]plumbing.Hash{}, map[string]*dir{}} }
	root := newDir()
	for p, h := range files {
		parts := strings.Split(p, "/")
		d := root
		for _, part := range parts[:len(parts)-1] {
			if d.dirs[part] == nil {
				d.dirs[part] = newDir()
			}
			d = d.dirs[part]
		}
		d.files[parts[len(parts)-1]] = h
	}
	var write func(d *dir) (plumbing.Hash, error)
	write = func(d *dir) (plumbing.Hash, error) {
		var entries []object.TreeEntry
		for n, h := range d.files {
			entries = append(entries, object.TreeEntry{Name: n, Mode: filemode.Regular, Hash: h})
		}
		for n, sub := range d.dirs {
			h, err := write(sub)
			if err != nil {
				return plumbing.ZeroHash, err
			}
			entries = append(entries, object.TreeEntry{Name: n, Mode: filemode.Dir, Hash: h})
		}
		// Git orders entries by name, comparing directories as if they end in "/".
		key := func(e object.TreeEntry) string {
			if e.Mode == filemode.Dir {
				return e.Name + "/"
			}
			return e.Name
		}
		sort.Slice(entries, func(i, j int) bool { return key(entries[i]) < key(entries[j]) })
		t := &object.Tree{Entries: entries}
		obj := r.g.Storer.NewEncodedObject()
		if err := t.Encode(obj); err != nil {
			return plumbing.ZeroHash, err
		}
		return r.g.Storer.SetEncodedObject(obj)
	}
	return write(root)
}

// CreateBranch creates branch name at the current commit of from.
func (r *Repo) CreateBranch(name, from string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, err := r.resolve(from)
	if err != nil {
		return err
	}
	ref := plumbing.NewBranchReferenceName(name)
	if _, err := r.g.Reference(ref, false); err == nil {
		return fmt.Errorf("branch %s already exists", name)
	}
	return r.g.Storer.SetReference(plumbing.NewHashReference(ref, h))
}

// DeleteBranch removes a branch. Its commits stay reachable from any tags.
func (r *Repo) DeleteBranch(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.g.Storer.RemoveReference(plumbing.NewBranchReferenceName(name))
}

// Merge writes a merge commit on into whose tree is into's tree with the
// given paths taken from branch from. Paths that do not exist on from are
// deleted. Both branches are parents, so history shows the merge.
func (r *Repo) Merge(into, from string, paths []string, a Author, msg string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	intoHash, err := r.resolve(into)
	if err != nil {
		return "", err
	}
	fromHash, err := r.resolve(from)
	if err != nil {
		return "", err
	}
	fc, err := r.g.CommitObject(fromHash)
	if err != nil {
		return "", err
	}
	ft, err := fc.Tree()
	if err != nil {
		return "", err
	}
	var changes []Change
	for _, p := range paths {
		f, err := ft.File(p)
		if err != nil {
			changes = append(changes, Change{Path: p, Delete: true})
			continue
		}
		b, err := f.Contents()
		if err != nil {
			return "", err
		}
		changes = append(changes, Change{Path: p, Content: []byte(b)})
	}
	h, err := r.commit(into, intoHash, changes, a, msg, []plumbing.Hash{fromHash})
	return h.String(), err
}

// Tag creates an annotated tag on a commit, signed when key is not nil.
func (r *Repo) Tag(name, commit string, a Author, msg string, key *openpgp.Entity) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, err := r.resolve(commit)
	if err != nil {
		return err
	}
	_, err = r.g.CreateTag(name, h, &git.CreateTagOptions{
		Tagger:  &object.Signature{Name: a.Name, Email: a.Email, When: time.Now()},
		Message: msg,
		SignKey: key,
	})
	return err
}

// Version is one commit that changed a file.
type Version struct {
	Commit  string
	Blob    string
	Author  string
	Email   string
	When    time.Time
	Message string
	Deleted bool
}

// History lists the commits that changed path, newest first, following
// first parents from ref. limit 0 means no limit.
func (r *Repo) History(ref, p string, limit int) ([]Version, error) {
	h, err := r.resolve(ref)
	if err != nil {
		return nil, err
	}
	var out []Version
	c, err := r.g.CommitObject(h)
	if err != nil {
		return nil, err
	}
	blobAt := func(c *object.Commit) plumbing.Hash {
		t, err := c.Tree()
		if err != nil {
			return plumbing.ZeroHash
		}
		e, err := t.FindEntry(p)
		if err != nil {
			return plumbing.ZeroHash
		}
		return e.Hash
	}
	cur := blobAt(c)
	for c != nil {
		var parent *object.Commit
		prev := plumbing.ZeroHash
		if len(c.ParentHashes) > 0 {
			parent, err = r.g.CommitObject(c.ParentHashes[0])
			if err != nil {
				return out, nil
			}
			prev = blobAt(parent)
		}
		if cur != prev {
			out = append(out, Version{
				Commit: c.Hash.String(), Blob: cur.String(), Author: c.Author.Name, Email: c.Author.Email,
				When: c.Author.When, Message: strings.TrimSpace(c.Message), Deleted: cur.IsZero(),
			})
			if limit > 0 && len(out) >= limit {
				break
			}
		}
		c, cur = parent, prev
	}
	return out, nil
}

// Blob returns the content of a blob by hash.
func (r *Repo) Blob(hash string) ([]byte, error) {
	b, err := r.g.BlobObject(plumbing.NewHash(hash))
	if err != nil {
		return nil, fmt.Errorf("blob %s: %w", hash, ErrNotFound)
	}
	rd, err := b.Reader()
	if err != nil {
		return nil, err
	}
	defer rd.Close()
	var buf bytes.Buffer
	_, err = io.Copy(&buf, rd)
	return buf.Bytes(), err
}

// Tags lists tag names with the given prefix.
func (r *Repo) Tags(prefix string) ([]string, error) {
	it, err := r.g.Tags()
	if err != nil {
		return nil, err
	}
	var out []string
	err = it.ForEach(func(ref *plumbing.Reference) error {
		n := ref.Name().Short()
		if strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// Changed lists the paths that differ between two refs.
func (r *Repo) Changed(fromRef, toRef string) ([]string, error) {
	a, err := r.tree(fromRef)
	if err != nil {
		return nil, err
	}
	b, err := r.tree(toRef)
	if err != nil {
		return nil, err
	}
	ch, err := object.DiffTree(a, b)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, c := range ch {
		for _, n := range []string{c.From.Name, c.To.Name} {
			if n != "" && !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// Repack packs loose objects to keep the repository small and fast.
func (r *Repo) Repack() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.g.RepackObjects(&git.RepackConfig{})
}
