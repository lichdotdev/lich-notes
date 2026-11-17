package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	noteExt       = ".md"
	untitledName  = "Untitled"
	noteFileMode  = 0o644
	hiddenDirMark = "."
)

var errOutsideVault = errors.New("note path leaves the vault")

// Note is one markdown file of a vault. Path is relative to the vault and
// slash-separated, so it is the same on every OS and safe as a node key.
type Note struct {
	Path     string
	Title    string
	Modified time.Time
}

// ListNotes walks vault for markdown files, newest first. Directories whose
// name starts with a dot hold an editor's own state, not notes.
func ListNotes(vault string) ([]Note, error) {
	var notes []Note
	err := filepath.WalkDir(vault, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != vault && strings.HasPrefix(d.Name(), hiddenDirMark) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), noteExt) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(vault, path)
		if err != nil {
			return err
		}
		notes = append(notes, Note{
			Path:     filepath.ToSlash(rel),
			Title:    strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())),
			Modified: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(notes, func(i, j int) bool { return notes[i].Modified.After(notes[j].Modified) })
	return notes, nil
}

// SearchNotes keeps the notes whose title or content holds query, ignoring
// case, in the order given. An empty query keeps them all.
//
// shortcut: reads every note on each call, fine for a few thousand files;
// build an index when a vault outgrows that.
func SearchNotes(vault string, notes []Note, query string) []Note {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return notes
	}
	var found []Note
	for _, n := range notes {
		if strings.Contains(strings.ToLower(n.Title), query) {
			found = append(found, n)
			continue
		}
		text, err := ReadNote(vault, n.Path)
		if err == nil && strings.Contains(strings.ToLower(text), query) {
			found = append(found, n)
		}
	}
	return found
}

func ReadNote(vault, rel string) (string, error) {
	path, err := notePath(vault, rel)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	return string(data), err
}

func SaveNote(vault, rel, text string) error {
	path, err := notePath(vault, rel)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), noteFileMode)
}

func DeleteNote(vault, rel string) error {
	path, err := notePath(vault, rel)
	if err != nil {
		return err
	}
	local, err := filepath.Rel(vault, path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Ext(local), noteExt) {
		return fmt.Errorf("not a Markdown note: %q", rel)
	}
	root, err := os.OpenRoot(vault)
	if err != nil {
		return err
	}
	defer root.Close()
	info, err := root.Lstat(local)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("not a Markdown note: %q", rel)
	}
	return root.Remove(local)
}

// CreateNote makes an empty "Untitled N.md" at the top of vault, N the first
// number no file has, and returns its path.
func CreateNote(vault string) (string, error) {
	for n := 1; ; n++ {
		name := fmt.Sprintf("%s %d%s", untitledName, n, noteExt)
		f, err := os.OpenFile(filepath.Join(vault, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, noteFileMode)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		return name, f.Close()
	}
}

// notePath refuses a path that would leave the vault: rel travels through
// lich inside a handler name and comes back in an event.
func notePath(vault, rel string) (string, error) {
	local := filepath.FromSlash(rel)
	if !filepath.IsLocal(local) {
		return "", fmt.Errorf("%w: %q", errOutsideVault, rel)
	}
	return filepath.Join(vault, local), nil
}
