package storage

import (
	"context"
	"log"
	"time"

	"polyglot/internal/data"
	pb "polyglot/proto/adapter"
)

// UiPathStorageService 实现 StorageService gRPC 接口（通用 KV 存储）。
// 类型名保留旧称以避免改动 server.go 调用点；实现已是框架无关的 KV。
type UiPathStorageService struct {
	pb.UnimplementedStorageServiceServer
	kv data.KVStoreRepository
}

// NewUiPathStorageServiceWithStore creates a KV-backed gRPC storage service.
func NewUiPathStorageServiceWithStore(store *data.Store) *UiPathStorageService {
	return &UiPathStorageService{kv: store.KVStore()}
}

// NewUiPathStorageServiceWithRepository creates a storage service from a KV repository.
func NewUiPathStorageServiceWithRepository(repo data.KVStoreRepository) *UiPathStorageService {
	return &UiPathStorageService{kv: repo}
}

// Close is a no-op: the KV storage service does not own the DB handle
// (the data store does, and is closed by the server).
func (s *UiPathStorageService) Close() error { return nil }

// Put 写入一条 KV 记录（value 不透明，由 adapter 自定义编码）。
func (s *UiPathStorageService) Put(ctx context.Context, req *pb.PutRequest) (*pb.PutResponse, error) {
	log.Printf("💾 KV Put: source=%s key=%s (%d bytes)", req.GetSourceId(), req.GetKey(), len(req.GetValue()))

	err := s.kv.Upsert(ctx, data.KVStoreRecord{
		SourceID:  req.GetSourceId(),
		Key:       req.GetKey(),
		Value:     req.GetValue(),
		ExpiresAt: req.GetExpiresAt(),
		UpdatedAt: time.Now().Unix(),
	})
	if err != nil {
		log.Printf("❌ KV Put failed: %v", err)
		return &pb.PutResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}
	return &pb.PutResponse{Success: true}, nil
}

// Get 按 (source_id, key) 读取。
func (s *UiPathStorageService) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	rec, found, err := s.kv.Get(ctx, req.GetSourceId(), req.GetKey())
	if err != nil {
		log.Printf("❌ KV Get failed: %v", err)
		return &pb.GetResponse{Found: false}, nil
	}
	if !found {
		return &pb.GetResponse{Found: false}, nil
	}
	return &pb.GetResponse{
		Found:     true,
		Value:     rec.Value,
		ExpiresAt: rec.ExpiresAt,
	}, nil
}

// Delete 按 (source_id, key) 删除。
func (s *UiPathStorageService) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	if err := s.kv.Delete(ctx, req.GetSourceId(), req.GetKey()); err != nil {
		log.Printf("❌ KV Delete failed: %v", err)
		return &pb.DeleteResponse{Success: false}, nil
	}
	return &pb.DeleteResponse{Success: true}, nil
}

// List 列出某 source_id 下的 keys（可选前缀过滤）。
func (s *UiPathStorageService) List(ctx context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	records, err := s.kv.List(ctx, req.GetSourceId(), req.GetPrefix())
	if err != nil {
		log.Printf("❌ KV List failed: %v", err)
		return &pb.ListResponse{}, nil
	}
	entries := make([]*pb.KVEntry, 0, len(records))
	for _, rec := range records {
		entries = append(entries, &pb.KVEntry{
			Key:       rec.Key,
			ExpiresAt: rec.ExpiresAt,
		})
	}
	return &pb.ListResponse{Entries: entries}, nil
}
