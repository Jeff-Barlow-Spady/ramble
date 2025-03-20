package embed

import (
	"os"
	"sync"
)

var (
	tempFiles     = make([]string, 0)
	tempFilesLock sync.Mutex
)

// RegisterTempFile adds a file to be cleaned up later
func RegisterTempFile(path string) {
	tempFilesLock.Lock()
	defer tempFilesLock.Unlock()
	tempFiles = append(tempFiles, path)
}

// CleanupTempFiles removes all temporary files
func CleanupTempFiles() {
	tempFilesLock.Lock()
	defer tempFilesLock.Unlock()

	for _, path := range tempFiles {
		os.RemoveAll(path)
	}
	tempFiles = tempFiles[:0]
}
