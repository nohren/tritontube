// Lab 7: Implement a SQLite video metadata service

package web

import (
	_ "github.com/mattn/go-sqlite3"
)

type SQLiteVideoMetadataService struct{}

// Uncomment the following line to ensure SQLiteVideoMetadataService implements VideoMetadataService
// var _ VideoMetadataService = (*SQLiteVideoMetadataService)(nil)
