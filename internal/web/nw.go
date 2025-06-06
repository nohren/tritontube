// Lab 8: Implement a network video content service (client using consistent hashing)

package web

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	storagepb "tritontube/internal/proto/storage"

	"google.golang.org/grpc"
)

// ConsistentHashRing interface
type ConsistentHashRing struct {
	// This struct would contain the necessary fields for a consistent hash ring.
	nodes []uint64          // Node identifiers (e.g., hash of node addresses)
	index map[uint64]string // Maps node identifiers to node addresses
	mu    sync.RWMutex      // Mutex to protect concurrent access to nodes and index

}

// NWVideoContentService implements VideoContentService using a network of nodes.
type NWVideoContentService struct {
	Ring         *ConsistentHashRing
	storageConns map[string]storagepb.StorageServiceClient
	mu           sync.Mutex // To protect concurrent access to storageConns
}

// Uncomment the following line to ensure NetworkVideoContentService implements VideoContentService
var _ VideoContentService = (*NWVideoContentService)(nil)

func ComposeKey(videoId, filename string) string {
	return fmt.Sprintf("%s/%s", videoId, filename)
}

// routes Read/Write calls over gRPC to one of N storage nodes via consistent hashing.
func NewNWVideoContentService(contentOptions string) *NWVideoContentService {
	// contentOptions is expected to look like:
	//   "adminhost:adminport,contenthost1:contentport1,contenthost2:contentport2,..."

	parts := strings.Split(contentOptions, ",")
	if len(parts) < 2 {
		panic(fmt.Sprintf("NWVideoContentService: need at least one admin and one node, got %q", contentOptions))
	}

	nodeAddrs := parts[1:]

	// Initialize the consistent hash ring
	// chunk nodes into a ring
	ring := NewConsistentHashRing()
	for _, addr := range nodeAddrs {
		ring.Add(addr) // Add each node's address to the ring as a consistent hash
	}

	//dial each node once and store the client stub
	conns := make(map[string]storagepb.StorageServiceClient, len(nodeAddrs))
	for _, addr := range nodeAddrs {
		//var opts []grpc.DialOption - no need for secure connection in this example
		clientConn, err := grpc.NewClient(addr, grpc.WithInsecure())
		if err != nil {
			panic(fmt.Sprintf("NWVideoContentService: failed to dial node %s: %v", addr, err))
		}
		// give grpc the conn, get the stub where we can call gRPC methods on that server
		conns[addr] = storagepb.NewStorageServiceClient(clientConn)
	}

	//dial each node once and store the client stub
	return &NWVideoContentService{
		Ring:         ring,
		storageConns: conns, // Store the gRPC clients for each node
	}
}

func (s *NWVideoContentService) DeleteFile(videoId string, filename string, nodeAddr string) error {
	//get the gRPC server stub
	client := s.storageConns[nodeAddr]

	// run gRPC call on server
	//fs operations at videoId/filename
	//baseDir provided by the server
	_, err := client.DeleteFile(context.Background(), &storagepb.DeleteRequest{
		Key: ComposeKey(videoId, filename), // use key as the identifier for the file
	})
	return err
}

// Identify the write node based on consistent hashing
// write to it
func (s *NWVideoContentService) Write(videoId, filename string, data []byte) error {
	key := fmt.Sprintf("%s/%s", videoId, filename) // Create a key based on videoId and filename
	nodeAddr, err := s.Ring.GetNodeForKey(key)     // Get the node address for the key
	if err != nil {
		return fmt.Errorf("no nodes in this ring.  failed to get node for key %s: %v", key, err)
	}

	//get the gRPC server stub
	client := s.storageConns[nodeAddr]

	// run gRPC call on server
	//fs operations at videoId/filename
	//baseDir provided by the server
	_, err = client.UploadFile(context.Background(), &storagepb.UploadRequest{
		Key:  key,  // use key as the identifier for the file
		Data: data, // data to write
	})
	return err
}

// ListChunks lists all chunks for a given server addr
// we need this for add and remove server so we can reassign chunks
func (s *NWVideoContentService) ListChunkNames(nodeAddr string) ([]string, error) {
	s.mu.Lock()
	client, exists := s.storageConns[nodeAddr]
	s.mu.Unlock()
	if !exists {
		return nil, fmt.Errorf("node %q not found in storage connections", nodeAddr)
	}
	// Call the ListFiles RPC on that storage node
	resp, err := client.ListFiles(context.Background(), &storagepb.ListFilesRequest{})
	if err != nil {
		return nil, fmt.Errorf("ListFiles RPC to %s failed: %v", nodeAddr, err)
	}

	return resp.GetKeys(), nil
}

// Read gets the content
func (s *NWVideoContentService) ReadFromNode(videoId, filename string, nodeAddr string) ([]byte, error) {
	key := fmt.Sprintf("%s/%s", videoId, filename) // Create a key based on videoId and filename

	//get the gRPC client for the node
	client := s.storageConns[nodeAddr]
	resp, err := client.DownloadFile(context.Background(), &storagepb.DownloadRequest{
		Key: key, // use key as the identifier for the file
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download file %s from node %s: %v", key, nodeAddr, err)
	}
	if !resp.Found {
		return nil, fmt.Errorf("file %s not found on node %s", key, nodeAddr)
	}
	return resp.Data, nil

}

// Read gets the content
func (s *NWVideoContentService) Read(videoId, filename string) ([]byte, error) {
	key := fmt.Sprintf("%s/%s", videoId, filename) // Create a key based on videoId and filename
	nodeAddr, err := s.Ring.GetNodeForKey(key)     // Get the node address for the key
	if err != nil {
		return nil, fmt.Errorf("no nodes in this ring.  failed to get node for key %s: %v", key, err)
	}

	//get the gRPC client for the node
	client := s.storageConns[nodeAddr]
	resp, err := client.DownloadFile(context.Background(), &storagepb.DownloadRequest{
		Key: key, // use key as the identifier for the file
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download file %s from node %s: %v", key, nodeAddr, err)
	}
	if !resp.Found {
		return nil, fmt.Errorf("file %s not found on node %s", key, nodeAddr)
	}
	return resp.Data, nil

}

func NewConsistentHashRing() *ConsistentHashRing {
	return &ConsistentHashRing{
		nodes: make([]uint64, 0), // init empty slice
		index: make(map[uint64]string),
	}
}

func hashStringToUint64(s string) uint64 {
	sum := sha256.Sum256([]byte(s))         // hash the string to a sha256 sum
	return binary.BigEndian.Uint64(sum[:8]) // convert the first 8 bytes of the hash to a uint64
}

// add a method to the struct consistentHashRing to add nodes.  The paraenthesis and r and pointer to the struct tells us this is a method of the struct we are defining
func (r *ConsistentHashRing) Add(nodeAddr string) {
	h := hashStringToUint64(nodeAddr) // hash the node address to a uint64
	r.mu.Lock()                       // lock the mutex to protect concurrent access
	defer r.mu.Unlock()               // unlock the mutex when the function returns
	r.index[h] = nodeAddr             // map the hash to the node address
	r.nodes = append(r.nodes, h)      // add the hash to the nodes slice
	sort.Slice(r.nodes, func(i, j int) bool {
		return r.nodes[i] < r.nodes[j] // sort the nodes slice in ascending order for each node hash of type int64
	})
}

// remove
func (r *ConsistentHashRing) Remove(nodeAddr string) {
	h := hashStringToUint64(nodeAddr) // hash the node address to a uint64
	r.mu.Lock()                       // not convinced this is needed, since there is only one admin server that modifies the ring
	// if we have multiple admin servers, i could see the need
	defer r.mu.Unlock()

	if _, exists := r.index[h]; !exists {
		fmt.Printf("Cannot remove node. Node %s not found in the ring", nodeAddr)
		return
	}
	delete(r.index, h)
	// Remove h from the slice
	for i, v := range r.nodes {
		if v == h {
			r.nodes = slices.Delete(r.nodes, i, i+1) // remove the hash from the nodes slice
			break
		}
	}
}

// given a hashed key h, find the smallest node n where n > h
func (r *ConsistentHashRing) GetNodeForKey(key string) (string, error) {
	h := hashStringToUint64(key) // hash the key to a uint64
	r.mu.RLock()                 // lock the mutex to protect concurrent access
	defer r.mu.RUnlock()         // unlock the mutex when the function returns

	if len(r.nodes) == 0 {
		return "", fmt.Errorf("no nodes available in the ring")
	}
	//r.nodes is sorted and monotonic. We can find the next higher node efficiently using binary search
	idx := sort.Search(len(r.nodes), func(i int) bool {
		return r.nodes[i] >= h // find the first node whose hash is greater than or equal to h
	})
	if idx == len(r.nodes) {
		// If idx is equal to the length of the nodes slice, it means h is greater than all node hashes
		// So we wrap around to the first node
		idx = 0
	}
	return r.index[r.nodes[idx]], nil // return the node address for the hash at index idx
}

func (r *ConsistentHashRing) List() []string {
	r.mu.RLock()         // lock the mutex to protect concurrent access
	defer r.mu.RUnlock() // unlock the mutex when the function returns

	nodes := make([]string, len(r.nodes))
	for i, h := range r.nodes {
		nodes[i] = r.index[h] // map the hash to the node address
	}
	return nodes
}
