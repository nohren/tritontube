package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	adminpb "tritontube/internal/proto"
	"tritontube/internal/web"

	"google.golang.org/grpc"
)

// printUsage prints the usage information for the application
func printUsage() {
	fmt.Println("Usage: ./program [OPTIONS] METADATA_TYPE METADATA_OPTIONS CONTENT_TYPE CONTENT_OPTIONS")
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  METADATA_TYPE         Metadata service type (sqlite, etcd)")
	fmt.Println("  METADATA_OPTIONS      Options for metadata service (e.g., db path)")
	fmt.Println("  CONTENT_TYPE          Content service type (fs, nw)")
	fmt.Println("  CONTENT_OPTIONS       Options for content service (e.g., base dir, network addresses)")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Example: ./program sqlite db.db fs /path/to/videos")
}

type AdminServer struct {
	adminpb.UnimplementedVideoContentAdminServiceServer
	svc *web.NWVideoContentService
	mu  sync.Mutex
}

func NewAdminServer(svc *web.NWVideoContentService) *AdminServer {
	return &AdminServer{
		svc: svc,
	}
}

func (a *AdminServer) ListNodes(ctx context.Context, req *adminpb.ListNodesRequest) (*adminpb.ListNodesResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	nodes := a.svc.Ring.List()
	response := &adminpb.ListNodesResponse{
		Nodes: nodes,
	}
	return response, nil
}

// add it to the ring
// read from each server in order to get the chunk names
// start migration process
// if chunk goes to server it came from, skip it
// if chunk goes to a different server, read it from the old server and write it to the new server, and delete it from the old server
// incremnt count
func (a *AdminServer) AddNode(ctx context.Context, req *adminpb.AddNodeRequest) (*adminpb.AddNodeResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// get the list of existing nodes in the cluster
	serverAddrs := a.svc.Ring.List()
	// Add the new node address to the consistent hash ring
	a.svc.Ring.Add(req.NodeAddress)

	if len(serverAddrs) == 0 {
		return nil, fmt.Errorf("no existing nodes in the cluster to migrate from")
	}

	migratedFileCount := int32(0)
	// Start migration process
	for _, addr := range serverAddrs {
		chunkNames, err := a.svc.ListChunkNames(addr)
		if err != nil {
			return nil, fmt.Errorf("failed to list chunk names for node %s: %w", req.NodeAddress, err)
		}

		for _, chunkName := range chunkNames {

			// (With ring updated) check if the chunk still belongs to the current addr
			chunkAddr, err := a.svc.Ring.GetNodeForKey(chunkName)
			if err != nil {
				return nil, fmt.Errorf("failed to get node for chunk %s: %w", chunkName, err)
			}
			if chunkAddr == addr {
				// leave chunk on the same server
				continue
			}
			// they are different, so we need to migrate the chunk
			// Read/Write/Delete the chunk
			parts := strings.Split(chunkName, "/")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid chunk name format %q, expected \"videoID/filename\"", chunkName)
			}
			videoId := parts[0]
			filename := parts[1]
			data, err := a.svc.ReadFromNode(videoId, filename, addr) // read the file from the old node
			if err != nil {
				return nil, fmt.Errorf("failed to read chunk %s from node %s: %w", chunkName, addr, err)
			}
			if err := a.svc.Write(videoId, filename, data); err != nil {
				return nil, fmt.Errorf("failed to write chunk %s to new node %s: %w", chunkName, req.NodeAddress, err)
			}
			// Now delete the file from the old node
			if err := a.svc.DeleteFile(videoId, filename, addr); err != nil {
				return nil, fmt.Errorf("failed to delete chunk %s from node %s: %w", chunkName, addr, err)
			}
			log.Printf("Migrated chunk %s from node %s to new node %s", chunkName, addr, req.NodeAddress)
			// Increment the migrated file count
			migratedFileCount++
		}
	}

	return &adminpb.AddNodeResponse{MigratedFileCount: migratedFileCount}, nil
}

func (a *AdminServer) RemoveNode(ctx context.Context, req *adminpb.RemoveNodeRequest) (*adminpb.RemoveNodeResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	//remove the node address from the consistent hash ring
	//a.svc.Ring.Remove(req.NodeAddress)

	// read all names from the videoId filename and data from the server being shut down
	chunkNames, err := a.svc.ListChunkNames(req.NodeAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to list chunk names for node %s: %w", req.NodeAddress, err)
	}
	//remove the node from the consistent hash ring
	//this ring is how video chunks find a storage home
	a.svc.Ring.Remove(req.NodeAddress)

	// start migration process
	// read from node being removed, write to new node, delete from node being removed
	migratedFileCount := int32(0)
	for _, chunkName := range chunkNames {
		parts := strings.Split(chunkName, "/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid chunk name format %q, expected \"videoID/filename\"", chunkNames)
		}
		videoId := parts[0]
		filename := parts[1]

		data, err := a.svc.ReadFromNode(videoId, filename, req.NodeAddress) // read the file from the node being removed
		if err != nil {
			return nil, fmt.Errorf("failed to read chunk %s from node %s: %w", chunkName, req.NodeAddress, err)
		}

		if err := a.svc.Write(videoId, filename, data); err != nil {
			return nil, fmt.Errorf("failed to write chunk %s to new node: %w", chunkName, err)
		}

		//now delete the file from the node being removed
		if err := a.svc.DeleteFile(videoId, filename, req.NodeAddress); err != nil {
			return nil, fmt.Errorf("failed to delete chunk %s from node %s: %w", chunkName, req.NodeAddress, err)
		}
		log.Printf("Migrated chunk %s from node %s to new node", chunkName, req.NodeAddress)
		migratedFileCount++
	}

	return &adminpb.RemoveNodeResponse{MigratedFileCount: migratedFileCount}, nil
}

func main() {
	// Define flags
	port := flag.Int("port", 8080, "Port number for the web server")
	host := flag.String("host", "localhost", "Host address for the web server")

	// Set custom usage message
	flag.Usage = printUsage

	// Parse flags
	flag.Parse()

	// Check if the correct number of positional arguments is provided
	if len(flag.Args()) != 4 {
		fmt.Println("Error: Incorrect number of arguments")
		printUsage()
		return
	}

	// Parse positional arguments
	metadataServiceType := flag.Arg(0)
	metadataServiceOptions := flag.Arg(1)
	contentServiceType := flag.Arg(2)
	contentServiceOptions := flag.Arg(3)

	// Validate port number (already an int from flag, check if positive)
	if *port <= 0 {
		fmt.Println("Error: Invalid port number:", *port)
		printUsage()
		return
	}

	// Construct metadata service
	var (
		metadataService web.VideoMetadataService
		err             error
	)
	fmt.Println("Creating metadata service of type", metadataServiceType, "with options", metadataServiceOptions)
	switch metadataServiceType {
	case "sqlite":
		// metadataServiceOptions is your file-path e.g. "meta.db"
		metadataService, err = web.NewSQLiteVideoMetadataService(metadataServiceOptions) //web.NewSQLiteVideoMetadataService is exported from web package and imported here "tritontube/internal/web"
		if err != nil {
			log.Fatalf("failed to open sqlite store %q: %v", metadataServiceOptions, err)
		}
	case "etcd":
		panic("Lab 9: not implemented")
	default:
		fmt.Println("Error: Unknown metadata service type:", metadataServiceType)
		return
	}

	// Construct content service
	var contentService web.VideoContentService
	fmt.Println("Creating content service of type", contentServiceType, "with options", contentServiceOptions)
	switch contentServiceType {
	case "fs":
		// contentServiceOptions is your base-dir e.g. "/path/to/videos"
		contentService = web.NewFSVideoContentService(contentServiceOptions)
	case "nw":
		// instantiate a network video content service
		contentService = web.NewNWVideoContentService(contentServiceOptions)

		// get listen address for gRPC and HTTP, first part is the admin address
		adminLstAddr := strings.Split(contentServiceOptions, ",")[0] //

		// Create a listener for gRPC and HTTP
		grpcL, err := net.Listen("tcp", adminLstAddr)
		if err != nil {
			log.Fatalf("failed to listen on %s: %v", adminLstAddr, err)
		}

		// start the admin service
		go func() {
			// Create your gRPC server
			grpcServer := grpc.NewServer()
			adminpb.RegisterVideoContentAdminServiceServer(grpcServer, NewAdminServer(contentService.(*web.NWVideoContentService)))

			fmt.Println("[gRPC] Admin server listening on", adminLstAddr)
			if err := grpcServer.Serve(grpcL); err != nil {
				log.Fatalf("gRPC (admin) Serve error: %v", err)
			}
		}()
	default:
		fmt.Println("Error: Unknown content service type:", contentServiceType)
		return
	}

	// Start the web server
	server := web.NewServer(metadataService, contentService)
	listenAddr := fmt.Sprintf("%s:%d", *host, *port)
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		fmt.Println("Error starting listener:", err)
		return
	}
	defer lis.Close()

	fmt.Println("Starting web server on", listenAddr)
	err = server.Start(lis)
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
}
