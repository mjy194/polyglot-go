package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"polyglot/internal/data"
	pb "polyglot/proto/adapter"
)

// UiPathStorageService 实现 StorageService gRPC 接口
type UiPathStorageService struct {
	pb.UnimplementedStorageServiceServer
	authStates data.AuthStateRepository
	closer     io.Closer
}

// NewUiPathStorageService 创建存储服务
func NewUiPathStorageService(dbPath string) (*UiPathStorageService, error) {
	store, err := data.Open(data.Config{
		Driver:      data.DriverSQLite,
		DSN:         dbPath,
		AutoMigrate: true,
	})
	if err != nil {
		return nil, err
	}
	return NewUiPathStorageServiceWithStore(store), nil
}

// NewUiPathStorageServiceWithStore creates a gRPC storage service from the data store.
func NewUiPathStorageServiceWithStore(store *data.Store) *UiPathStorageService {
	return &UiPathStorageService{
		authStates: store.AuthStates(),
	}
}

// NewUiPathStorageServiceWithRepository creates a storage service from a repository.
func NewUiPathStorageServiceWithRepository(repo data.AuthStateRepository) *UiPathStorageService {
	return &UiPathStorageService{authStates: repo}
}

// SaveAuthState 保存认证状态
func (s *UiPathStorageService) SaveAuthState(ctx context.Context, req *pb.SaveAuthStateRequest) (*pb.SaveAuthStateResponse, error) {
	log.Printf("💾 SaveAuthState: email=%s\n", req.Email)

	// 构建存储格式（对齐 Rust 版本）
	sessionKey := "default"
	accountKey := fmt.Sprintf("account:%s", req.Email)

	// 先加载现有状态
	var storeData map[string]interface{}
	record, found, err := s.authStates.Get(ctx, "default")

	if err != nil {
		return &pb.SaveAuthStateResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to load existing state: %v", err),
		}, nil
	}
	if !found {
		// 初始化新结构
		storeData = map[string]interface{}{
			"version":        2,
			"active_account": accountKey,
			"accounts":       map[string]interface{}{},
		}
	} else if err := json.Unmarshal([]byte(record.Value), &storeData); err != nil {
		return &pb.SaveAuthStateResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to parse existing state: %v", err),
		}, nil
	}

	// 更新 accounts 结构
	accounts := storeData["accounts"].(map[string]interface{})
	if accounts[accountKey] == nil {
		accounts[accountKey] = map[string]interface{}{
			"label":    req.Email,
			"sessions": map[string]interface{}{},
		}
	}

	account := accounts[accountKey].(map[string]interface{})
	account["active_session"] = sessionKey

	sessions := account["sessions"].(map[string]interface{})
	sessions[sessionKey] = map[string]interface{}{
		"access_token":  req.AccessToken,
		"refresh_token": req.RefreshToken,
		"expires_at":    req.ExpiresAt,
		"upstream_url":  req.UpstreamUrl,
	}

	// 序列化并保存
	raw, err := json.Marshal(storeData)
	if err != nil {
		return &pb.SaveAuthStateResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to marshal state: %v", err),
		}, nil
	}

	now := time.Now().Unix()
	err = s.authStates.Upsert(ctx, data.AuthStateRecord{
		Key:       "default",
		Value:     string(raw),
		UpdatedAt: now,
	})

	if err != nil {
		return &pb.SaveAuthStateResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to save to database: %v", err),
		}, nil
	}

	log.Printf("✅ Auth state saved successfully\n")
	return &pb.SaveAuthStateResponse{
		Success: true,
		Message: "Auth state saved",
	}, nil
}

// LoadAuthState 加载认证状态
func (s *UiPathStorageService) LoadAuthState(ctx context.Context, req *pb.LoadAuthStateRequest) (*pb.LoadAuthStateResponse, error) {
	log.Printf("📂 LoadAuthState: email=%s\n", req.Email)

	sessionKey := "default"
	accountKey := fmt.Sprintf("account:%s", req.Email)

	record, found, err := s.authStates.Get(ctx, "default")
	if err != nil {
		log.Printf("❌ Failed to query database: %v\n", err)
		return &pb.LoadAuthStateResponse{Found: false}, nil
	}
	if !found {
		log.Println("⚠️  No auth state found in database")
		return &pb.LoadAuthStateResponse{Found: false}, nil
	}

	// 解析 JSON
	var storeData map[string]interface{}
	if err := json.Unmarshal([]byte(record.Value), &storeData); err != nil {
		log.Printf("❌ Failed to parse auth state: %v\n", err)
		return &pb.LoadAuthStateResponse{Found: false}, nil
	}

	// 导航到 session
	accounts, ok := storeData["accounts"].(map[string]interface{})
	if !ok {
		return &pb.LoadAuthStateResponse{Found: false}, nil
	}

	account, ok := accounts[accountKey].(map[string]interface{})
	if !ok {
		return &pb.LoadAuthStateResponse{Found: false}, nil
	}

	sessions, ok := account["sessions"].(map[string]interface{})
	if !ok {
		return &pb.LoadAuthStateResponse{Found: false}, nil
	}

	session, ok := sessions[sessionKey].(map[string]interface{})
	if !ok {
		return &pb.LoadAuthStateResponse{Found: false}, nil
	}

	// 提取字段
	accessToken, _ := session["access_token"].(string)
	refreshToken, _ := session["refresh_token"].(string)
	upstreamURL, _ := session["upstream_url"].(string)

	var expiresAt int64
	switch v := session["expires_at"].(type) {
	case float64:
		expiresAt = int64(v)
	case int64:
		expiresAt = v
	}

	// 检查是否过期
	if expiresAt > 0 && time.Now().Unix() > expiresAt {
		log.Println("⚠️  Token expired")
		return &pb.LoadAuthStateResponse{Found: false}, nil
	}

	log.Println("✅ Auth state loaded successfully")
	return &pb.LoadAuthStateResponse{
		Found:        true,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		UpstreamUrl:  upstreamURL,
	}, nil
}

// Close 关闭数据库连接
func (s *UiPathStorageService) Close() error {
	if s == nil || s.closer == nil {
		return nil
	}
	return s.closer.Close()
}
