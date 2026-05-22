//go:build windows

package storage

import "os"

// Windows builds rely on the process-local mutex in LocalStorage.
// This keeps local development working even though syscall.Flock is unavailable.
func lockExclusive(_ *os.File) error {
	return nil
}

func unlockExclusive(_ *os.File) error {
	return nil
}
