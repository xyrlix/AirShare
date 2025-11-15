package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"airshare-backend/internal/config"
	"airshare-backend/internal/discovery"
	"airshare-backend/internal/transfer"
	"airshare-backend/pkg/models"
	"github.com/gorilla/websocket"
)

// Server HTTP服务器
type Server struct {
	config           *config.ServerConfig
	discoveryService *discovery.DiscoveryManager
	transferService  *transfer.Service
	upgrader         websocket.Upgrader
	clients         map[*websocket.Conn]bool
	clientMutex     sync.RWMutex
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
	// 设置路由
	http.HandleFunc("/", s.handleRoot)
	http.HandleFunc("/api/devices", s.handleDevices)
	http.HandleFunc("/api/transfer", s.handleTransfer)
	http.HandleFunc("/ws", s.handleWebSocket)

	// 启动文件服务
	if s.config.WebRoot != "" {
		http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(s.config.WebRoot))))
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	log.Printf("服务器启动在 %s", addr)
	
	return http.ListenAndServe(addr, nil)
}

// Stop 停止服务器
func (s *Server) Stop() {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	// 关闭所有WebSocket连接
	for client := range s.clients {
		client.Close()
	}
	s.clients = make(map[*websocket.Conn]bool)
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

// handleDevices 处理设备列表请求
func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	// 暂时返回空设备列表，因为s.discoveryService.GetDevices方法未定义
	// devices := s.discoveryService.GetDevices()
	s.sendJSONResponse(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    []string{},
	})
}

// handleTransfer 处理文件传输请求
func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		// 暂时返回一个占位响应，因为handleTransferStart方法未定义
		s.sendJSONResponse(w, http.StatusOK, models.APIResponse{
			Success: true,
			Message: "Transfer functionality not fully implemented",
		})
	case "GET":
		// 暂时返回一个占位响应，因为handleTransferStatus方法未定义
		s.sendJSONResponse(w, http.StatusOK, models.APIResponse{
			Success: true,
			Message: "Transfer status functionality not fully implemented",
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
	// 暂时返回空设备列表，因为s.discoveryService.GetDevices方法未定义
	// devices := s.discoveryService.GetDevices()
	msg := models.WebSocketMessage{
		Type: models.MessageTypeDeviceList,
		Data: []string{},
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
			"message": "传输功能正在开发中",
		},
	}
	// 直接使用websocket的WriteJSON方法
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
	defer s.clientMutex.RUnlock()

	for client := range s.clients {
		if err := client.WriteJSON(msg); err != nil {
			log.Printf("广播消息失败: %v", err)
			// 移除失败的客户端
			go func(c *websocket.Conn) {
				s.clientMutex.Lock()
				delete(s.clients, c)
				s.clientMutex.Unlock()
				c.Close()
			}(client)
		}
	}
}