// Lab 7: Implement a SQLite video metadata service

package web

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mattn/go-sqlite3"
)

type SQLiteVideoMetadataService struct {
	db *sql.DB
}

var (
	ErrVideoExists   = errors.New("video already exists")
	ErrVideoNotFound = errors.New("video not found")
)

func NewSQLiteVideoMetadataService(dbPath string) (*SQLiteVideoMetadataService, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// TODO: What is the schema?
	// create table if it does not exist
	schema := `CREATE TABLE IF NOT EXISTS videos (
		id TEXT PRIMARY KEY,
		uploaded_at DATETIME NOT NULL
	);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLiteVideoMetadataService{db: db}, nil
}

func (s *SQLiteVideoMetadataService) Create(videoId string, uploadedAt time.Time) error {
	// Create inserts a new video metadata record into the database.
	_, err := s.db.Exec(
		`INSERT INTO videos (id, uploaded_at) VALUES (?, ?)`,
		videoId, uploadedAt,
	)
	if err != nil {
		if sqliteErr, ok := err.(sqlite3.Error); ok && sqliteErr.Code == sqlite3.ErrConstraint {
			return ErrVideoExists // If the video already exists, return a specific error
		} else {
			return fmt.Errorf("sqlite insert failed: %w", err)
		}
	}
	return err
}
func (s *SQLiteVideoMetadataService) List() ([]VideoMetadata, error) {
	// Query all videos ordered by upload time descending
	rows, err := s.db.Query(
		`SELECT id, uploaded_at FROM videos ORDER BY uploaded_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vids []VideoMetadata
	//iterate over the rows, create a VideoMetadata struct for each row
	for rows.Next() {
		var id string
		var uploadedAt time.Time
		if err := rows.Scan(&id, &uploadedAt); err != nil {
			return nil, err
		}
		vids = append(vids, VideoMetadata{Id: id, UploadedAt: uploadedAt})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return vids, nil
}

// Return a VideoMetadata object associated with the given ID. Return nil if there is no entry associated with the ID.
func (s *SQLiteVideoMetadataService) Read(id string) (*VideoMetadata, error) {
	row := s.db.QueryRow(
		`SELECT id, uploaded_at FROM videos WHERE id = ?`,
		id,
	)
	// declare an empty VideoMetadata struct, for the row scan copy the values into the struct
	// if there is no row, return nil
	// if there is an error, return the error
	// if there is a row, return the struct
	var v VideoMetadata
	if err := row.Scan(&v.Id, &v.UploadedAt); err != nil {
		return nil, err
	}
	return &v, nil
}

// Uncomment the following line to ensure SQLiteVideoMetadataService implements VideoMetadataService
// means that SQLiteVideoMetadataService implements all methods of the VideoMetadataService interface
var _ VideoMetadataService = (*SQLiteVideoMetadataService)(nil)
