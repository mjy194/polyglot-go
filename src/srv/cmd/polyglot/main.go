package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"polyglot/internal/config"
	"polyglot/internal/server"
	"polyglot/pkg/logger"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	// 打印版本信息
	fmt.Printf("🌐 Polyglot - Universal AI API Gateway\n")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Build: %s\n", BuildTime)
	fmt.Printf("Commit: %s\n\n", GitCommit)

	// 加载配置
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	log := logger.New(cfg.Log)
	defer log.Sync()

	// 创建服务器
	srv, err := server.New(cfg, log)
	if err != nil {
		log.Fatal("Failed to create server", "error", err)
	}

	// 启动服务器（异步）
	go func() {
		if err := srv.Run(); err != nil {
			log.Fatal("Server error", "error", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", "error", err)
	}

	log.Info("Server stopped")
}
