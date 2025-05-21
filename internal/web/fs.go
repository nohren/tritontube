// Lab 7: Implement a local filesystem video content service

package web

// FSVideoContentService implements VideoContentService using the local filesystem.
type FSVideoContentService struct{}

//video content service interface signatures
//Read(videoId string, filename string) ([]byte, error)
//Write(videoId string, filename string, data []byte) error

// Uncomment the following line to ensure FSVideoContentService implements VideoContentService
var _ VideoContentService = (*FSVideoContentService)(nil)
