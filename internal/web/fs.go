// Lab 7: Implement a local filesystem video content service
// Separation of concerns, this handles I/O only for the file system operations
// video file conversion encoding is handled in the server

package web

import (
	"os"
	"path/filepath"
)

// FSVideoContentService implements VideoContentService using the local filesystem.
type FSVideoContentService struct {
	baseDir string
}

//video content service interface signatures
//Read(videoId string, filename string) ([]byte, error)
//Write(videoId string, filename string, data []byte) error

// Compile-time assertion that FSVideoContentService implements VideoContentService
var _ VideoContentService = (*FSVideoContentService)(nil)

// NewFSVideoContentService returns a filesystem-based content service rooted at baseDir.
func NewFSVideoContentService(baseDir string) *FSVideoContentService {
	return &FSVideoContentService{baseDir: baseDir}
}

// Write stores the given data under baseDir/videoId/filename.
func (s *FSVideoContentService) Write(videoId, filename string, data []byte) error {

	dir := filepath.Join(s.baseDir, videoId)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 2) If your filename ever contains sub-folders
	//    (e.g. "segments/chunk-00001.m4s"), ensure *that* path exists too:
	fullPath := filepath.Join(dir, filename)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(fullPath, data, 0644)
}

// Read retrieves the content data for the specified video and filename.
func (s *FSVideoContentService) Read(videoId, filename string) ([]byte, error) {
	path := filepath.Join(s.baseDir, videoId, filename)
	return os.ReadFile(path)
}
