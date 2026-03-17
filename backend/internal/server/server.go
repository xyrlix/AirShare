package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"airshare-backend/internal/config"
	"airshare-backend/internal/discovery"
	"airshare-backend/internal/transfer"
	"airshare-backend/pkg/models"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// Server HTTP服务器
type Server struct {
	config           *config.ServerConfig
	discoveryService *discovery.DiscoveryManager
	transferService  *transfer.Service
	upgrader         websocket.Upgrader
	clients          map[*websocket.Conn]bool
	clientMutex      sync.RWMutex
	httpServer       *http.Server
}

// New 创建新的服务器
func New(serverConfig *config.ServerConfig, discoveryService *discovery.DiscoveryManager, transferService *transfer.Service) *Server {
	return &Server{
		config:           serverConfig,
		discoveryService: discoveryService,
		transferService:  transferService,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源，生产环境需要严格限制
			},
		},
		clients: make(map[*websocket.Conn]bool),
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	// 使用 gorilla/mux 创建独立路由器
	router := mux.NewRouter()

	// API 路由
	api := router.PathPrefix("/api").Subrouter()

	// 设备相关路由
	api.HandleFunc("/devices", s.handleGetDevices).Methods("GET")

	// 传输相关路由
	api.HandleFunc("/transfer", s.handleSendFile).Methods("POST")
	api.HandleFunc("/transfer/history", s.handleGetTransferHistory).Methods("GET")
	api.HandleFunc("/transfer/{transfer_id}", s.handleGetTransferStatus).Methods("GET")
	api.HandleFunc("/transfer/{transfer_id}/cancel", s.handleCancelTransfer).Methods("POST")
	api.HandleFunc("/transfer/{transfer_id}/pause", s.handlePauseTransfer).Methods("POST")
	api.HandleFunc("/transfer/{transfer_id}/resume", s.handleResumeTransfer).Methods("POST")
	api.HandleFunc("/transfer/{transfer_id}/file/{filename}", s.handleDownloadFile).Methods("GET")

	// 文件相关路由
	api.HandleFunc("/files", s.handleGetFiles).Methods("GET")
	api.HandleFunc("/files/{filename}", s.handleDeleteFile).Methods("DELETE")

	// 健康检查
	api.HandleFunc("/health", s.handleHealth).Methods("GET")
	api.HandleFunc("/status", s.handleStatus).Methods("GET")

	// WebSocket 路由
	router.HandleFunc("/ws", s.handleWebSocket)

	// 根路由
	router.HandleFunc("/", s.handleRoot)

	// 静态文件服务
	if s.config.WebRoot != "" {
		router.PathPrefix("/static/").Handler(
			http.StripPrefix("/static/", http.FileServer(http.Dir(s.config.WebRoot))),
		)
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	log.Printf("服务器启动在 %s", addr)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s.httpServer.ListenAndServe()
}

// Stop 停止服务器
func (s *Server) Stop() {
	// 关闭所有WebSocket连接
	s.clientMutex.Lock()
	for client := range s.clients {
		client.Close()
	}
	s.clients = make(map[*websocket.Conn]bool)
	s.clientMutex.Unlock()

	// 优雅关闭 HTTP 服务器
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("服务器关闭失败: %v", err)
		}
	}
}

// handleRoot 处理根路径
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if s.config.WebRoot != "" {
		http.ServeFile(w, r, s.config.WebRoot+"/index.html")
	} else {
		s.sendJSONResponse(w, http.StatusOK, models.APIResponse{
			Success: true,
			Message: "AirShare API Server",
		})
	}
}

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.sendJSONResponse(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "OK",
	})
}

// handleStatus 服务状态
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	stats := s.discoveryService.GetStats()
	s.sendJSONResponse(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    stats,
	})
}

// handleDevices 处理设备列表请求（旧兼容路由）
func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	devices := s.discoveryService.GetOnlineDevices()
	s.sendJSONResponse(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    devices,
	})
}

// handleTransfer 处理文件传输请求（旧兼容路由）
func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		s.handleSendFile(w, r)
	case "GET":
		s.sendJSONResponse(w, http.StatusOK, models.APIResponse{
			Success: true,
			Message: "Transfer status functionality",
		})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleWebSocketMessages 处理WebSocket消息
func (s *Server) handleWebSocketMessages(conn *websocket.Conn) {
	defer func() {
		// 客户端断开连接
		s.clientMutex.Lock()
		delete(s.clients, conn)
		s.clientMutex.Unlock()
		conn.Close()
		log.Printf("WebSocket连接断开: %s", conn.RemoteAddr())
	}()

	for {
		var msg models.WebSocketMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket读取错误: %v", err)
			}
			break
		}

		s.handleWebSocketMessage(conn, &msg)
	}
}

// handleWebSocketMessage 处理单个WebSocket消息
func (s *Server) handleWebSocketMessage(conn *websocket.Conn, msg *models.WebSocketMessage) {
	switch msg.Type {
	case models.MessageTypeDeviceList:
		s.sendDeviceList(conn)
	case models.MessageTypeTransfer:
		s.handleTransferMessage(conn, msg)
	case models.MessageTypeKeepAlive:
		// 心跳包，不做任何处理
	default:
		log.Printf("未知的消息类型: %s", msg.Type)
		s.sendError(conn, "未知的消息类型")
	}
}

// sendDeviceList 发送设备列表
func (s *Server) sendDeviceList(conn *websocket.Conn) {
	devices := s.discoveryService.GetOnlineDevices()
	msg := models.WebSocketMessage{
		Type: models.MessageTypeDeviceList,
		Data: devices,
	}

	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("发送设备列表失败: %v", err)
	}
}

// handleTransferMessage 处理传输消息
func (s *Server) handleTransferMessage(conn *websocket.Conn, msg *models.WebSocketMessage) {
	// 这里需要根据消息内容处理文件传输
	// 实际实现需要处理文件分片、进度更新等
	responseMsg := models.WebSocketMessage{
		Type: models.MessageTypeProgress,
		Data: map[string]interface{}{
			"message": "传输功能正在处理中",
		},
	}
	if err := conn.WriteJSON(responseMsg); err != nil {
		log.Printf("发送传输消息失败: %v", err)
	}
}

// sendError 发送错误消息
func (s *Server) sendError(conn *websocket.Conn, errorMsg string) {
	msg := models.WebSocketMessage{
		Type:  models.MessageTypeError,
		Error: errorMsg,
	}

	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("发送错误消息失败: %v", err)
	}
}

// sendJSONResponse 发送JSON响应
func (s *Server) sendJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("JSON编码失败: %v", err)
	}
}

// broadcastToClients 广播消息给所有客户端
func (s *Server) broadcastToClients(msg models.WebSocketMessage) {
	s.clientMutex.RLock()
	clients := make([]*websocket.Conn, 0, len(s.clients))
	for client := range s.clients {
		clients = append(clients, client)
	}
	s.clientMutex.RUnlock()

	for _, client := range clients {
		if err := client.WriteJSON(msg); err != nil {
			log.Printf("广播消息失败: %v", err)
			// 移除失败的客户端
			s.clientMutex.Lock()
			delete(s.clients, client)
			s.clientMutex.Unlock()
			client.Close()
		}
	}
}
