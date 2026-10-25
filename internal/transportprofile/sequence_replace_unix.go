//go:build !windows

package transportprofile

import "os"

// replaceSequenceFile uses rename's atomic replacement semantics on Unix-like
// systems because the temporary file is created in the destination directory.
func replaceSequenceFile(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
