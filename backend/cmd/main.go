package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AirShare/backend/internal/config"
	"github.com/AirShare/backend/internal/discovery"
	"github.com/AirShare/backend/internal/server"
	"github.com/AirShare/backend/internal/transfer"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "path to config file")
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 初始化服务
	discoveryService := discovery.NewService(cfg)
	transferService := transfer.NewService(cfg)
	server := server.New(cfg, discoveryService, transferService)

	// 启动服务
	errCh := make(chan error, 3)

	// 启动设备发现服务
	go func() {
		if err := discoveryService.Start(ctx); err != nil {
			errCh <- fmt.Errorf("discovery service error: %v", err)
		}
	}()

	// 启动文件传输服务
	go func() {
		if err := transferService.Start(ctx); err != nil {
			errCh <- fmt.Errorf("transfer service error: %v", err)
		}
	}()

	// 启动HTTP服务器
	go func() {
		log.Printf("Starting server on %s", cfg.Server.Address)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("server error: %v", err)
		}
	}()

	// 等待信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("Received signal: %v", sig)
	case err := <-errCh:
		log.Printf("Service error: %v", err)
	}

	// 优雅关闭
	log.Println("Shutting down services...")
	
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	if err := transferService.Stop(shutdownCtx); err != nil {
		log.Printf("Transfer service stop error: %v", err)
	}

	if err := discoveryService.Stop(shutdownCtx); err != nil {
		log.Printf("Discovery service stop error: %v", err)
	}

	log.Println("AirShare server stopped successfully")
}