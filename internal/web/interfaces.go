package web

import "time"

type VideoMetadata struct { //VideoMetadata is a struct that holds metadata for a video
	Id         string
	UploadedAt time.Time
}

type VideoMetadataService interface { //VideoMetadataService is an interface that defines methods for managing video metadata
	Read(id string) (*VideoMetadata, error)
	List() ([]VideoMetadata, error)
	Create(videoId string, uploadedAt time.Time) error
}

type VideoContentService interface { //VideoContentService is an interface that defines methods for managing video content
	Read(videoId string, filename string) ([]byte, error)
	Write(videoId string, filename string, data []byte) error
}
