package usecase

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/Frank5ive/goforge/internal/domain"
)

// changeset collects every file a command will create or edit, so they can be
// written together and undone together if a later step fails. Nothing touches
// the filesystem until apply.
type changeset struct {
	writer  domain.FileWriter
	creates map[string]string // path -> content, for files that must not exist yet
	edits   map[string]string // path -> new content, for existing files
	order   []string          // paths in the order they were added
	notes   []string          // follow-up steps for the user
}

func newChangeset(writer domain.FileWriter) *changeset {
	return &changeset{
		writer:  writer,
		creates: map[string]string{},
		edits:   map[string]string{},
	}
}

// create queues a new file. It fails if the file already exists.
func (c *changeset) create(path, content string) error {
	if c.writer.FileExists(path) {
		return domain.WithCode(domain.CodeAlreadyExists, fmt.Errorf("%s already exists", path))
	}
	if _, queued := c.creates[path]; !queued {
		c.order = append(c.order, path)
	}
	c.creates[path] = content
	return nil
}

// edit queues new content for an existing file.
func (c *changeset) edit(path, content string) {
	if _, queued := c.edits[path]; !queued {
		c.order = append(c.order, path)
	}
	c.edits[path] = content
}

// apply writes every queued file, then runs after (if not nil) for steps that
// must see the new files, such as updating dependencies. If anything fails,
// created files are removed and edited files, plus any paths in alsoRestore,
// get their original content back. Paths in alsoRestore that did not exist
// before (such as a go.sum that `go get` creates) are removed.
func (c *changeset) apply(alsoRestore []string, after func() error) error {
	originals := map[string]string{}
	var missing []string
	for _, path := range append(c.editPaths(), alsoRestore...) {
		if !c.writer.FileExists(path) {
			missing = append(missing, path)
			continue
		}
		content, err := c.writer.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s before changing it: %w", path, err)
		}
		originals[path] = content
	}

	// Directories that don't exist yet are created by WriteFile, so a rollback
	// must remove them too
	newDirs := c.missingDirs()

	var written []string
	rollback := func(cause error) error {
		for _, path := range written {
			if _, created := c.creates[path]; created {
				_ = c.writer.RemoveAll(path)
			}
		}
		for _, dir := range newDirs {
			_ = c.writer.RemoveAll(dir)
		}
		for _, path := range missing {
			_ = c.writer.RemoveAll(path)
		}
		for path, content := range originals {
			_ = c.writer.WriteFile(path, content)
		}
		return cause
	}

	for _, path := range c.order {
		content, created := c.creates[path]
		if !created {
			content = c.edits[path]
		}
		if err := c.writer.WriteFile(path, content); err != nil {
			return rollback(fmt.Errorf("writing %s: %w", path, err))
		}
		written = append(written, path)
	}

	if after != nil {
		if err := after(); err != nil {
			return rollback(err)
		}
	}
	return nil
}

// missingDirs returns the top-most directories of queued new files that don't
// exist yet, i.e. the directories applying the changeset will create.
func (c *changeset) missingDirs() []string {
	seen := map[string]bool{}
	var dirs []string
	for path := range c.creates {
		top := ""
		for dir := filepath.Dir(path); dir != filepath.Dir(dir) && !c.writer.FileExists(dir); dir = filepath.Dir(dir) {
			top = dir
		}
		if top != "" && !seen[top] {
			seen[top] = true
			dirs = append(dirs, top)
		}
	}
	sort.Strings(dirs)
	return dirs
}

// note records a follow-up step the command could not do itself.
func (c *changeset) note(format string, args ...any) {
	c.notes = append(c.notes, fmt.Sprintf(format, args...))
}

// createdIn returns the queued new files under dir, relative to it, sorted.
func (c *changeset) createdIn(dir string) []string {
	return c.relative(dir, c.creates)
}

// editedIn returns the queued edited files under dir, relative to it, sorted.
func (c *changeset) editedIn(dir string) []string {
	return c.relative(dir, c.edits)
}

func (c *changeset) editPaths() []string {
	paths := make([]string, 0, len(c.edits))
	for path := range c.edits {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func (c *changeset) relative(dir string, files map[string]string) []string {
	var paths []string
	for path := range files {
		if rel, err := filepath.Rel(dir, path); err == nil {
			paths = append(paths, filepath.ToSlash(rel))
		}
	}
	sort.Strings(paths)
	return paths
}
