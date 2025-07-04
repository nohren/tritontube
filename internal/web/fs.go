// Lab 7: Implement a local filesystem video content service
// Separation of concerns, this handles I/O only for the file system operations
// video file conversion encoding is handled in the server

package web

import (
	"fmt"
	"log"
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
	// create the base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		log.Print(err)
	}
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

func (s *FSVideoContentService) Delete(videoId, filename string) error {
	path := filepath.Join(s.baseDir, videoId, filename)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil // File does not exist, return nil
		}
		return err // Return any other error
	}
	return nil // Successfully deleted
}

func (s *FSVideoContentService) ListAll() ([]string, error) {
	var keys []string
	err := filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Compute the path relative to baseDir
		rel, err := filepath.Rel(s.baseDir, path)
		if err != nil {
			return err
		}
		// Normalize to forward slashes
		key := filepath.ToSlash(rel)
		keys = append(keys, key)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ListFiles walk error: %v", err)
	}
	return keys, nil
}
