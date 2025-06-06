[![Review Assignment Due Date](https://classroom.github.com/assets/deadline-readme-button-22041afd0340ce965d47ae6ef1cefeee28c7c493a6346c4f15d667ab976d596c.svg)](https://classroom.github.com/a/e5W8wwsN)

# Docs

https://docs.google.com/document/d/1izFnibGVxzNRgpKl3RKt6_4gy64cN0ZEX8_GPLniasE/edit?tab=t.x8vofnrgqex1#heading=h.tee8x89zfyvi

# Launch server

<!-- `go run ./cmd/web/main.go -port 8080 sqlite meta.db fs ./storage` -->

`go run ./cmd/storage -host localhost -port 8090 "./storage/8090"`

```
mkdir -p storage/8090 storage/8091 storage/8092

go run ./cmd/storage -port 8090 "./storage/8090" # storage 8090
go run ./cmd/storage -port 8091 "./storage/8091" # storage 8091
go run ./cmd/storage -port 8092 "./storage/8092" # storage 8092

go run ./cmd/web \
    sqlite "./metadata.db" \
    nw     "localhost:8081,localhost:8090,localhost:8091,localhost:8092"


```

```
go run ./cmd/admin add localhost:8081 localhost:8090
go run ./cmd/admin remove localhost:8081 localhost:8090
go run ./cmd/admin list localhost:8081
```

# gRPC

```
protoc \
  --proto_path=proto \
  --go_out=internal/proto/storage --go_opt=paths=source_relative \
  --go-grpc_out=internal/proto/storage --go-grpc_opt=paths=source_relative \
  proto/storage.proto


```

# Project 8 - Distributed video storage

Files pertaining to lab 8

nw.go
server.go
proto files

Objectives for lab 8

- modify ./cmd/web/main.go to support the nw video content type...
  - nw, it takes a string in the following format as CONTENT_OPTIONS: adminhost:adminport,contenthost1:contentport1,contenthost2:contentport2
  - first server in csv's, is the web server, the rest are storage servers nobo
- upload - Replace FSVideoContentService with NWVideoContentService - which saves each file to one of the storage servers using consistent hashing.
- view - When a user requests a video file, NWVideoContentService use consistent hashing to find it and retrieve it.
- adding/removing storage servers (fault tolerancy?) - done via admin cli - admin cli sends a request to the gRPC server running in tandem with HTTP server. This updates the list of storage servers and web server need to migrate files based on updated hash ring. Basically to accurate react to topology change.
  - this seems more about moving files around and changing hash ring. Not about stopping and starting servers.

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

see nw.go

## interfaces / webserver

VideoMetadataService, VideoContentService

FSVideoContentService - implements the VideoContentService interface using the local filesystem

modify internal/web/fs.go - make FSVideoContentService compatible with VideoMetadataService

`./cmd/web/main.go` - run server

Need to provide four positional arguments METADATA_TYPE, METADATA_OPTIONS, CONTENT_TYPE, and CONTENT_OPTIONS

When METADATA_TYPE is set to sqlite, METADATA_OPTIONS specifies the path to the database file

When CONTENT_TYPE is set to fs, CONTENT_OPTIONS specifies the directory to save files. For Read and Write operations, create a directory for each video ID. For example, store manifest.mpd for CSE-X24 as {directory}/CSE-X24/manifest.mpd.

Also host and port
