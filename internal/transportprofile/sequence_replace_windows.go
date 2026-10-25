//go:build windows

package transportprofile

import (
	"syscall"
	"unsafe"
)

var (
	kernel32SequenceStore = syscall.NewLazyDLL("kernel32.dll")
	moveFileExSequence    = kernel32SequenceStore.NewProc("MoveFileExW")
)

const (
	moveFileReplaceExisting = 0x1
	moveFileWriteThrough    = 0x8
)

// replaceSequenceFile uses MoveFileExW with replace-existing and write-through
// flags because os.Rename does not replace an existing destination on Windows.
func replaceSequenceFile(oldPath, newPath string) error {
	oldName, err := syscall.UTF16PtrFromString(oldPath)
	if err != nil {
		return err
	}
	newName, err := syscall.UTF16PtrFromString(newPath)
	if err != nil {
		return err
	}
	result, _, callErr := moveFileExSequence.Call(
		uintptr(unsafe.Pointer(oldName)),
		uintptr(unsafe.Pointer(newName)),
		moveFileReplaceExisting|moveFileWriteThrough,
	)
	if result != 0 {
		return nil
	}
	if callErr != syscall.Errno(0) {
		return callErr
	}
	return syscall.EINVAL
}
