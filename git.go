package main

// Minimal git implementation using only the standard library.
// Writes loose objects, a branch ref and a version 2 index, so the data
// folder is a normal repository that any git client can read.

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var gitMu sync.Mutex

type Git struct{ dir string } // working tree root

func (g Git) gd(p ...string) string { return filepath.Join(append([]string{g.dir, ".git"}, p...)...) }

func (g Git) Init() error {
	if _, err := os.Stat(g.gd("HEAD")); err == nil {
		return nil
	}
	for _, d := range []string{"objects", filepath.Join("refs", "heads"), filepath.Join("refs", "tags")} {
		if err := os.MkdirAll(g.gd(d), 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(g.gd("HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		return err
	}
	cfg := "[core]\n\trepositoryformatversion = 0\n\tfilemode = false\n\tbare = false\n\tautocrlf = false\n"
	return os.WriteFile(g.gd("config"), []byte(cfg), 0o644)
}

func (g Git) headRef() string {
	b, err := os.ReadFile(g.gd("HEAD"))
	if err != nil {
		return "refs/heads/main"
	}
	s := strings.TrimSpace(string(b))
	if strings.HasPrefix(s, "ref: ") {
		return strings.TrimPrefix(s, "ref: ")
	}
	return "refs/heads/main"
}

// Head returns the current commit id, or "" for an empty repository.
func (g Git) Head() string {
	ref := g.headRef()
	if b, err := os.ReadFile(g.gd(filepath.FromSlash(ref))); err == nil {
		return strings.TrimSpace(string(b))
	}
	if f, err := os.Open(g.gd("packed-refs")); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			parts := strings.Fields(sc.Text())
			if len(parts) == 2 && parts[1] == ref {
				return parts[0]
			}
		}
	}
	return ""
}

func (g Git) writeObject(kind string, data []byte) (string, error) {
	hdr := fmt.Sprintf("%s %d\x00", kind, len(data))
	h := sha1.New()
	h.Write([]byte(hdr))
	h.Write(data)
	id := hex.EncodeToString(h.Sum(nil))
	p := g.gd("objects", id[:2], id[2:])
	if _, err := os.Stat(p); err == nil {
		return id, nil
	}
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	zw.Write([]byte(hdr))
	zw.Write(data)
	zw.Close()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o444); err != nil {
		return "", err
	}
	return id, os.Rename(tmp, p)
}

func (g Git) readObject(id string) (string, []byte, error) {
	if len(id) != 40 {
		return "", nil, errors.New("bad object id")
	}
	f, err := os.Open(g.gd("objects", id[:2], id[2:]))
	if err != nil {
		return "", nil, fmt.Errorf("object %s not available (the repository may have been packed by another git tool)", id[:8])
	}
	defer f.Close()
	zr, err := zlib.NewReader(f)
	if err != nil {
		return "", nil, err
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		return "", nil, err
	}
	nul := bytes.IndexByte(raw, 0)
	if nul < 0 {
		return "", nil, errors.New("corrupt object")
	}
	kind := strings.SplitN(string(raw[:nul]), " ", 2)[0]
	return kind, raw[nul+1:], nil
}

type fileEntry struct {
	path string // slash separated, relative
	id   string
	info os.FileInfo
}

type treeNode struct {
	files map[string]string
	dirs  map[string]*treeNode
}

func (g Git) writeTree(n *treeNode) (string, error) {
	type ent struct {
		name, sortKey, mode, id string
	}
	var ents []ent
	for name, id := range n.files {
		ents = append(ents, ent{name, name, "100644", id})
	}
	for name, sub := range n.dirs {
		id, err := g.writeTree(sub)
		if err != nil {
			return "", err
		}
		ents = append(ents, ent{name, name + "/", "40000", id})
	}
	sort.Slice(ents, func(i, j int) bool { return ents[i].sortKey < ents[j].sortKey })
	var buf bytes.Buffer
	for _, e := range ents {
		raw, _ := hex.DecodeString(e.id)
		buf.WriteString(e.mode + " " + e.name + "\x00")
		buf.Write(raw)
	}
	return g.writeObject("tree", buf.Bytes())
}

// CommitAll snapshots every file in the working tree (except .git) and commits it.
func (g Git) CommitAll(author, msg string) (string, error) {
	gitMu.Lock()
	defer gitMu.Unlock()
	var files []fileEntry
	err := filepath.Walk(g.dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() || strings.HasSuffix(info.Name(), ".tmp") {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		id, err := g.writeObject("blob", data)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(g.dir, p)
		files = append(files, fileEntry{filepath.ToSlash(rel), id, info})
		return nil
	})
	if err != nil {
		return "", err
	}
	root := &treeNode{map[string]string{}, map[string]*treeNode{}}
	for _, f := range files {
		parts := strings.Split(f.path, "/")
		n := root
		for _, d := range parts[:len(parts)-1] {
			if n.dirs[d] == nil {
				n.dirs[d] = &treeNode{map[string]string{}, map[string]*treeNode{}}
			}
			n = n.dirs[d]
		}
		n.files[parts[len(parts)-1]] = f.id
	}
	tree, err := g.writeTree(root)
	if err != nil {
		return "", err
	}
	parent := g.Head()
	if parent != "" {
		if _, pdata, err := g.readObject(parent); err == nil {
			if t := commitField(pdata, "tree"); t == tree {
				return parent, nil // nothing changed
			}
		}
	}
	now := time.Now()
	sig := fmt.Sprintf("%s <%s@local> %d %s", author, author, now.Unix(), now.Format("-0700"))
	var c bytes.Buffer
	c.WriteString("tree " + tree + "\n")
	if parent != "" {
		c.WriteString("parent " + parent + "\n")
	}
	c.WriteString("author " + sig + "\ncommitter " + sig + "\n\n" + strings.TrimSpace(msg) + "\n")
	id, err := g.writeObject("commit", c.Bytes())
	if err != nil {
		return "", err
	}
	ref := g.gd(filepath.FromSlash(g.headRef()))
	os.MkdirAll(filepath.Dir(ref), 0o755)
	if err := os.WriteFile(ref, []byte(id+"\n"), 0o644); err != nil {
		return "", err
	}
	g.writeIndex(files)
	return id, nil
}

func (g Git) writeIndex(files []fileEntry) {
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	var buf bytes.Buffer
	buf.WriteString("DIRC")
	binary.Write(&buf, binary.BigEndian, uint32(2))
	binary.Write(&buf, binary.BigEndian, uint32(len(files)))
	for _, f := range files {
		start := buf.Len()
		mt := f.info.ModTime()
		for _, v := range []uint32{uint32(mt.Unix()), uint32(mt.Nanosecond()), uint32(mt.Unix()), uint32(mt.Nanosecond()), 0, 0, 0o100644, 0, 0, uint32(f.info.Size())} {
			binary.Write(&buf, binary.BigEndian, v)
		}
		raw, _ := hex.DecodeString(f.id)
		buf.Write(raw)
		n := len(f.path)
		if n > 0xFFF {
			n = 0xFFF
		}
		binary.Write(&buf, binary.BigEndian, uint16(n))
		buf.WriteString(f.path)
		for pad := 8 - (buf.Len()-start)%8; pad > 0; pad-- {
			buf.WriteByte(0)
		}
	}
	sum := sha1.Sum(buf.Bytes())
	buf.Write(sum[:])
	os.WriteFile(g.gd("index"), buf.Bytes(), 0o644)
}

func commitField(data []byte, field string) string {
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			break
		}
		if strings.HasPrefix(line, field+" ") {
			return strings.TrimPrefix(line, field+" ")
		}
	}
	return ""
}

type Commit struct {
	ID, Author, Message string
	When                time.Time
	Parent              string
}

func (g Git) ReadCommit(id string) (Commit, string, error) {
	kind, data, err := g.readObject(id)
	if err != nil {
		return Commit{}, "", err
	}
	if kind != "commit" {
		return Commit{}, "", errors.New("not a commit")
	}
	c := Commit{ID: id, Parent: commitField(data, "parent")}
	a := commitField(data, "author")
	if lt := strings.LastIndex(a, " <"); lt > 0 {
		c.Author = a[:lt]
		f := strings.Fields(a[strings.LastIndex(a, ">")+1:])
		if len(f) == 2 {
			var sec int64
			fmt.Sscan(f[0], &sec)
			c.When = time.Unix(sec, 0)
		}
	}
	if i := bytes.Index(data, []byte("\n\n")); i >= 0 {
		c.Message = strings.TrimSpace(string(data[i+2:]))
	}
	return c, commitField(data, "tree"), nil
}

// blobAt finds the blob id of a slash path inside a tree.
func (g Git) blobAt(tree, path string) string {
	cur := tree
	parts := strings.Split(path, "/")
	for i, part := range parts {
		_, data, err := g.readObject(cur)
		if err != nil {
			return ""
		}
		found := ""
		for len(data) > 0 {
			sp := bytes.IndexByte(data, ' ')
			nul := bytes.IndexByte(data, 0)
			if sp < 0 || nul < 0 || nul+21 > len(data) {
				return ""
			}
			name := string(data[sp+1 : nul])
			id := hex.EncodeToString(data[nul+1 : nul+21])
			data = data[nul+21:]
			if name == part {
				found = id
				break
			}
		}
		if found == "" {
			return ""
		}
		if i == len(parts)-1 {
			return found
		}
		cur = found
	}
	return ""
}

func (g Git) ReadBlob(id string) string {
	if id == "" {
		return ""
	}
	_, data, err := g.readObject(id)
	if err != nil {
		return ""
	}
	return string(data)
}

type FileVersion struct {
	Commit
	Blob, PrevBlob string
}

// FileHistory walks first parents from HEAD and returns commits that changed path.
func (g Git) FileHistory(path string, limit int) ([]FileVersion, error) {
	var out []FileVersion
	id := g.Head()
	var pending *FileVersion
	for steps := 0; id != "" && steps < 5000 && len(out) < limit; steps++ {
		c, tree, err := g.ReadCommit(id)
		if err != nil {
			if pending != nil {
				out = append(out, *pending)
			}
			return out, err
		}
		blob := g.blobAt(tree, path)
		if pending != nil {
			if pending.Blob != blob {
				pending.PrevBlob = blob
				out = append(out, *pending)
				pending = nil
			}
		}
		if pending == nil && blob != "" {
			pending = &FileVersion{Commit: c, Blob: blob}
		} else if pending != nil && pending.Blob == blob {
			pending.Commit = c // same content further back: attribute to the older commit
		}
		id = c.Parent
	}
	if pending != nil {
		out = append(out, *pending)
	}
	return out, nil
}
