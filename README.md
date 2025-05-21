[![Review Assignment Due Date](https://classroom.github.com/assets/deadline-readme-button-22041afd0340ce965d47ae6ef1cefeee28c7c493a6346c4f15d667ab976d596c.svg)](https://classroom.github.com/a/e5W8wwsN)

# Launch server

`go run ./cmd/web/main.go -port 8080 sqlite meta.db fs ./storage`

# Project 7 - SQLite and Local FS

Files pertaining to lab 7
internal/

- fs.go
- server.go
- SQLite.go
- templates.go

web/

- main.go

Objectives for lab 7

-

# High level

Website to upload videos.

Tasks

Metadata store

- Implement SQLiteVideoMetadataService in internal/web/sqlite.go as described earlier in this doc. Support the sqlite metadata type in cmd/web/main.go
  Content Store
- Implement FSVideoContentService in internal/web/fs.go as described earlier in this doc. Support the fs metadata type in cmd/web/main.go.
  Handlers
- Implement 4 HTTP endpoints in internal/web/server.go.

## Test

`mkdir -p ./storage`
`go run ./cmd/web/main.go -port 8080 sqlite ./metadata.db fs ./storage`

From there, interact with the service’s UI and make sure that you can:
See the landing page with the upload form and button present
Upload a video
Click the link to open the video page
Stream video on the video page

## Endpoints

GET /

POST /upload

GET /videos/:videold

GET /content/:videold/:filename

## Video Streaming

MPEG-DASH
.mpd files

## Non-persistent storage

SQ-lite to store metadata

For this project, it is sufficient to create a single table that can store video IDs and upload dates

## interfaces / webserver

VideoMetadataService, VideoContentService

FSVideoContentService - implements the VideoContentService interface using the local filesystem

modify internal/web/fs.go - make FSVideoContentService compatible with VideoMetadataService

`./cmd/web/main.go` - run server

Need to provide four positional arguments METADATA_TYPE, METADATA_OPTIONS, CONTENT_TYPE, and CONTENT_OPTIONS

When METADATA_TYPE is set to sqlite, METADATA_OPTIONS specifies the path to the database file

When CONTENT_TYPE is set to fs, CONTENT_OPTIONS specifies the directory to save files. For Read and Write operations, create a directory for each video ID. For example, store manifest.mpd for CSE-X24 as {directory}/CSE-X24/manifest.mpd.

Also host and port
