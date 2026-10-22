package filesystem

import "github.com/Frank5ive/goforge/internal/domain"

// RecordingWriter implements domain.FileWriter by wrapping another FileWriter and
// recording the path of every file written. In dry-run mode it records paths
// without modifying the filesystem, while FileExists still checks the real one.
type RecordingWriter struct {
	inner  domain.FileWriter
	dryRun bool
	files  []string
}

// NewRecordingWriter creates a RecordingWriter around inner.
func NewRecordingWriter(inner domain.FileWriter, dryRun bool) *RecordingWriter {
	return &RecordingWriter{inner: inner, dryRun: dryRun}
}

// WriteFile implements domain.FileWriter.WriteFile.
func (w *RecordingWriter) WriteFile(path string, content string) error {
	if !w.dryRun {
		if err := w.inner.WriteFile(path, content); err != nil {
			return err
		}
	}

	w.files = append(w.files, path)
	return nil
}

// CreateDirectory implements domain.FileWriter.CreateDirectory.
func (w *RecordingWriter) CreateDirectory(path string) error {
	if w.dryRun {
		return nil
	}
	return w.inner.CreateDirectory(path)
}

// FileExists implements domain.FileWriter.FileExists.
// It always checks the real filesystem, so dry runs report existing projects.
func (w *RecordingWriter) FileExists(path string) bool {
	return w.inner.FileExists(path)
}

// RemoveAll implements domain.FileWriter.RemoveAll.
func (w *RecordingWriter) RemoveAll(path string) error {
	if w.dryRun {
		return nil
	}
	return w.inner.RemoveAll(path)
}

// Files returns the paths written so far, in order.
func (w *RecordingWriter) Files() []string {
	return append([]string(nil), w.files...)
}

// DryRun reports whether the writer skips filesystem changes.
func (w *RecordingWriter) DryRun() bool {
	return w.dryRun
}
