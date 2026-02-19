package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"airshare-backend/internal/config"
	"airshare-backend/internal/discovery"
	"airshare-backend/internal/server"
	"airshare-backend/internal/transfer"
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

	// 创建上下文（暂时未使用）

	// 初始化服务
	// 使用默认参数创建discoveryManager
	discoveryManager := discovery.NewDiscoveryManager(5*time.Second, 30*time.Second, 100)
	transferService, err := transfer.NewService(&cfg.Transfer)
	if err != nil {
		log.Fatalf("Failed to create transfer service: %v", err)
	}
	server := server.New(&cfg.Server, discoveryManager, transferService)

	// 启动服务
	errCh := make(chan error, 3)

	// 启动设备发现服务
	// 注意：Start方法不接受参数
	go func() {
		discoveryManager.Start()
	}()

	// 启动文件传输服务
	// transferService没有Start方法，我们直接跳过
	go func() {
		// 传输服务不需要启动
	}()

	// 启动HTTP服务器
	go func() {
		log.Printf("Starting server on %s:%d", cfg.Server.Host, cfg.Server.Port)
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
	// 由于我们不使用shutdownCtx，直接使用defer关闭

	// 停止服务器
	server.Stop()

	// 停止传输服务
	transferService.Stop()

	// 停止设备发现服务
	discoveryManager.Stop()

	log.Println("AirShare server stopped successfully")
}