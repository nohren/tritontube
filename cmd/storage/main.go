package main

// by the time we reach this code, we already know what server and file we want to make the video client work for MPEG-DASH streaming

//This is for running these gRPC servers stubs on the server side

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	storagepb "tritontube/internal/proto/storage"
	"tritontube/internal/web"

	"google.golang.org/grpc"
)

type server struct {
	storagepb.UnimplementedStorageServiceServer
	fs *web.FSVideoContentService
}

//gRPC - remote procedure call bodies

func decomposeKey(key string) (videoId, filename string, err error) {
	// Assuming the key is structured as "videoId/filename"
	parts := strings.SplitN(key, "/", 2) // split into at most 2 pieces
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid key format %q, expected \"videoID/filename\"", key)
	}
	return parts[0], parts[1], nil
}

// UploadFile
// this is a method of the server struct
func (s *server) UploadFile(ctx context.Context, req *storagepb.UploadRequest) (*storagepb.UploadResponse, error) {
	// Implement file upload logic here
	//log.Printf("Received file upload request for file: %s", req.Key)
	key := req.Key
	data := req.GetData() //[]bytes

	videoId, filename, err := decomposeKey(key)
	if err != nil {
		log.Printf("Error decomposing key %s: %v", key, err)
		return &storagepb.UploadResponse{
			Success: false}, fmt.Errorf("invalid key format %q, expected \"videoID/filename\": %w", key, err)
	}

	if err := s.fs.Write(videoId, filename, data); err != nil {
		return &storagepb.UploadResponse{
			Success: false}, err
	}
	return &storagepb.UploadResponse{Success: true}, nil
}

// DownloadFile
func (s *server) DownloadFile(ctx context.Context, req *storagepb.DownloadRequest) (*storagepb.DownloadResponse, error) {
	key := req.Key
	//log.Printf("Received file download request for file: %s", key)

	videoId, filename, err := decomposeKey(key)
	if err != nil {
		log.Printf("Error decomposing key %s: %v", key, err)
		return &storagepb.DownloadResponse{
			Found: false,
			Data:  nil}, fmt.Errorf("invalid key format %q, expected \"videoID/filename\": %w", key, err)
	}
	data, err := s.fs.Read(videoId, filename)
	if err != nil {
		return &storagepb.DownloadResponse{
			Found: false,
			Data:  nil}, fmt.Errorf("file not found %s/%s", videoId, filename) // return not found
	}

	return &storagepb.DownloadResponse{
		Found: true,
		Data:  data,
	}, nil
}

// DeleteFile
func (s *server) DeleteFile(ctx context.Context, req *storagepb.DeleteRequest) (*storagepb.DeleteResponse, error) {
	key := req.Key

	videoId, filename, err := decomposeKey(key)
	if err != nil {
		log.Printf("Error decomposing key %s: %v", key, err)
		return &storagepb.DeleteResponse{
			Success: false}, fmt.Errorf("invalid key format %q, expected \"videoID/filename\": %w", key, err)
	}
	// log.Printf("Received file delete request for file: %s", key)
	if err := s.fs.Delete(videoId, filename); err != nil {
		return &storagepb.DeleteResponse{
			Success: false}, err
	}
	return &storagepb.DeleteResponse{Success: true}, nil
}

func main() {
	host := flag.String("host", "localhost", "Host address for the server")
	port := flag.Int("port", 8090, "Port number for the server")
	flag.Parse()

	// Validate arguments
	if *port <= 0 {
		panic("Error: Port number must be positive")
	}

	if flag.NArg() < 1 {
		fmt.Println("Usage: storage [OPTIONS] <baseDir>")
		fmt.Println("Error: Base directory argument is required")
		return
	}
	baseDir := flag.Arg(0)

	fmt.Println("Starting storage server...")
	fmt.Printf("Host: %s\n", *host)
	fmt.Printf("Port: %d\n", *port)
	fmt.Printf("Base Directory: %s\n", baseDir)

	// Create a new server instance
	//fmt.Sprintf is used to format a string, or come up with a new string
	addr := fmt.Sprintf("%s:%d", *host, *port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	// new gRPC server
	grpcServer := grpc.NewServer()
	fs_svc := web.NewFSVideoContentService(baseDir)
	storagepb.RegisterStorageServiceServer(grpcServer, &server{fs: fs_svc})
	log.Printf("Storage server listening on %s; storing files under %s", addr, baseDir)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}
