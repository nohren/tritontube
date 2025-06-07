// Lab 9: Implement a distributed video metadata service

package web

import (
	"context"
	"sort"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdVideoMetadataService struct {
	client *clientv3.Client // The client for interacting with etcd
	prefix string           // The prefix for the keys in etcd

}

// NewEtcdVideoMetadataService returns a new EtcdVideoMetadataService instance.
func NewEtcdVideoMetadataService(endpoints string) (*EtcdVideoMetadataService, error) {
	eps := strings.Split(endpoints, ",")
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   eps,
		DialTimeout: 5 * time.Second, // Set a reasonable timeout for connecting to etcd
	})
	if err != nil {
		return nil, err
	}
	return &EtcdVideoMetadataService{
		client: cli,
		prefix: "videos/", // Define a prefix for video metadata keys}
	}, nil
}

// Uncomment the following line to ensure EtcdVideoMetadataService implements VideoMetadataService
var _ VideoMetadataService = (*EtcdVideoMetadataService)(nil)

// key/value store, we prefix the keys with "videos/" to avoid conflicts with other keys in etcd.
// the value is the upload time in RFC3339 format, which is a standard format for representing date and time.

// this differes from a relational database where we would have a table with columns for video ID and upload time.
func (s *EtcdVideoMetadataService) Create(videoId string, uploadedAt time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Set a timeout for the operation
	defer cancel()                                                          // Ensure the context is cancelled to free resources
	key := s.prefix + videoId
	val := uploadedAt.Format(time.RFC3339Nano) // Store the time in RFC3339 format
	txn := s.client.Txn(ctx)
	// Use a transaction to ensure atomicity
	resp, err := txn.If(
		clientv3.Compare(clientv3.CreateRevision(key), "=", 0), // Check if the key does not exist)
	).Then(
		clientv3.OpPut(key, val), // If the key does not exist, put the new key-value pair
	).Commit() // Commit the transaction
	if err != nil {
		return err
	}
	if !resp.Succeeded { // If the transaction did not succeed, it means the key already exists
		return ErrVideoExists
	}
	return nil
}

func (s *EtcdVideoMetadataService) Read(videoId string) (*VideoMetadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Set a timeout for the operation
	defer cancel()                                                          // Ensure the context is cancelled to free resources
	key := s.prefix + videoId
	resp, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 { // If no value is found for the key, return nil
		return nil, ErrVideoNotFound
	}
	raw := string(resp.Kvs[0].Value)
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil { // If the value cannot be parsed as a time, return an error
		return nil, err
	}
	return &VideoMetadata{Id: videoId, UploadedAt: t}, nil
}

func (s *EtcdVideoMetadataService) List() ([]VideoMetadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Set a timeout for the operation
	defer cancel()                                                          // Ensure the context is cancelled to free resources
	resp, err := s.client.Get(ctx, s.prefix, clientv3.WithPrefix())         // Get all keys with the prefix "videos/"
	if err != nil {
		return nil, err
	}
	vids := make([]VideoMetadata, 0, len(resp.Kvs)) // for each key-value pair in /videos, create a VideoMetadata struct
	for _, kv := range resp.Kvs {
		videoId := strings.TrimPrefix(string(kv.Key), s.prefix)  // Remove the prefix to get the video ID
		t, err := time.Parse(time.RFC3339Nano, string(kv.Value)) // Parse the value as a time
		if err != nil {
			return nil, err
		}
		vids = append(vids, VideoMetadata{Id: videoId, UploadedAt: t}) // Append the video metadata to the slice
	}
	sort.Slice(vids, func(i, j int) bool {
		return vids[i].UploadedAt.After(vids[j].UploadedAt)
	})
	return vids, nil
}
