package transfer

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketServer 实现WebSocket通信服务
type WebSocketServer struct {
	upgrader websocket.Upgrader
	clients  map[string]*WebSocketClient
	mu       sync.RWMutex
	handlers map[string]MessageHandler
}

// WebSocketClient 表示WebSocket客户端连接
type WebSocketClient struct {
	ID         string
	conn       *websocket.Conn
	sendChan   chan []byte
	closeChan  chan bool
	lastActive time.Time
}

// MessageHandler WebSocket消息处理器
type MessageHandler func(client *WebSocketClient, msg WebSocketMessage) error

// WebSocketMessage WebSocket消息格式
type WebSocketMessage struct {
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp int64                  `json:"timestamp"`
	SessionID string                 `json:"session_id,omitempty"`
}

// 消息类型常量
const (
	MessageTypeConnect     = "connect"
	MessageTypeDisconnect  = "disconnect"
	MessageTypePing        = "ping"
	MessageTypePong        = "pong"
	MessageTypeFileInfo    = "file_info"
	MessageTypeTransfer    = "transfer"
	MessageTypeProgress    = "progress"
	MessageTypeError       = "error"
	MessageTypeDiscovery   = "discovery"
	MessageTypeDeviceInfo  = "device_info"
)

// NewWebSocketServer 创建新的WebSocket服务器
func NewWebSocketServer() *WebSocketServer {
	return &WebSocketServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// 允许所有来源的连接（生产环境应更严格）
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		clients:  make(map[string]*WebSocketClient),
		handlers: make(map[string]MessageHandler),
	}
}

// RegisterHandler 注册消息处理器
func (s *WebSocketServer) RegisterHandler(messageType string, handler MessageHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[messageType] = handler
}

// HandleWebSocket WebSocket连接处理入口
func (s *WebSocketServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}

	clientID := generateClientID()
	client := &WebSocketClient{
		ID:         clientID,
		conn:       conn,
		sendChan:   make(chan []byte, 100),
		closeChan:  make(chan bool),
		lastActive: time.Now(),
	}

	// 注册客户端
	s.mu.Lock()
	s.clients[clientID] = client
	s.mu.Unlock()

	log.Printf("客户端 %s 已连接", clientID)

	// 启动消息处理协程
	go s.handleClientMessages(client)
	go s.handleClientSend(client)

	// 发送连接成功消息
	s.sendWelcomeMessage(client)
}

// 处理客户端消息
func (s *WebSocketServer) handleClientMessages(client *WebSocketClient) {
	defer s.cleanupClient(client)

	for {
		select {
		case <-client.closeChan:
			return
		default:
			var msg WebSocketMessage
			err := client.conn.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("客户端 %s 读取消息错误: %v", client.ID, err)
				}
				return
			}

			// 更新最后活跃时间
			client.lastActive = time.Now()

			// 处理消息
			s.processMessage(client, msg)
		}
	}
}

// 处理客户端发送消息
func (s *WebSocketServer) handleClientSend(client *WebSocketClient) {
	defer s.cleanupClient(client)

	for {
		select {
		case <-client.closeChan:
			return
		case msg := <-client.sendChan:
			err := client.conn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Printf("客户端 %s 发送消息错误: %v", client.ID, err)
				return
			}
		}
	}
}

// 处理消息
func (s *WebSocketServer) processMessage(client *WebSocketClient, msg WebSocketMessage) {
	s.mu.RLock()
	handler, exists := s.handlers[msg.Type]
	s.mu.RUnlock()

	if !exists {
		log.Printf("未知消息类型: %s", msg.Type)
		s.sendError(client, "unknown_message_type", "未知的消息类型")
		return
	}

	if err := handler(client, msg); err != nil {
		log.Printf("处理消息 %s 失败: %v", msg.Type, err)
		s.sendError(client, "message_processing_error", err.Error())
	}
}

// 发送欢迎消息
func (s *WebSocketServer) sendWelcomeMessage(client *WebSocketClient) {
	msg := WebSocketMessage{
		Type:      "welcome",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"client_id": client.ID,
			"message":   "欢迎连接到AirShare文件传输服务",
			"version":   "1.0.0",
		},
	}

	s.sendMessage(client, msg)
}

// 发送错误消息
func (s *WebSocketServer) sendError(client *WebSocketClient, errorCode, message string) {
	msg := WebSocketMessage{
		Type:      MessageTypeError,
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"code":    errorCode,
			"message": message,
		},
	}

	s.sendMessage(client, msg)
}

// 发送消息到客户端
func (s *WebSocketServer) sendMessage(client *WebSocketClient, msg WebSocketMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("序列化消息失败: %v", err)
		return
	}

	select {
	case client.sendChan <- data:
		// 消息已排队
	case <-time.After(5 * time.Second):
		log.Printf("发送消息到客户端 %s 超时", client.ID)
	}
}

// 广播消息到所有客户端
func (s *WebSocketServer) Broadcast(msg WebSocketMessage) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("序列化广播消息失败: %v", err)
		return
	}

	for _, client := range s.clients {
		select {
		case client.sendChan <- data:
			// 消息已排队
		case <-time.After(1 * time.Second):
			log.Printf("广播消息到客户端 %s 超时", client.ID)
		}
	}
}

// 发送消息到特定客户端
func (s *WebSocketServer) SendToClient(clientID string, msg WebSocketMessage) error {
	s.mu.RLock()
	client, exists := s.clients[clientID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("客户端不存在: %s", clientID)
	}

	s.sendMessage(client, msg)
	return nil
}

// 清理客户端资源
func (s *WebSocketServer) cleanupClient(client *WebSocketClient) {
	// 关闭连接
	if client.conn != nil {
		client.conn.Close()
	}

	// 从客户端列表中移除
	s.mu.Lock()
	delete(s.clients, client.ID)
	s.mu.Unlock()

	// 关闭通道
	close(client.closeChan)
	close(client.sendChan)

	log.Printf("客户端 %s 已断开连接", client.ID)

	// 广播设备离线消息
	s.Broadcast(WebSocketMessage{
		Type:      MessageTypeDisconnect,
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"client_id": client.ID,
			"message":   "设备已离线",
		},
	})
}

// 获取活动客户端列表
func (s *WebSocketServer) GetActiveClients() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var clients []string
	for clientID := range s.clients {
		clients = append(clients, clientID)
	}

	return clients
}

// 获取客户端信息
func (s *WebSocketServer) GetClientInfo(clientID string) (*WebSocketClient, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, exists := s.clients[clientID]
	return client, exists
}

// 生成客户端ID
func generateClientID() string {
	return fmt.Sprintf("client_%d", time.Now().UnixNano())
}

// 默认消息处理器
func (s *WebSocketServer) registerDefaultHandlers() {
	s.RegisterHandler(MessageTypePing, s.handlePing)
	s.RegisterHandler(MessageTypeConnect, s.handleConnect)
	s.RegisterHandler(MessageTypeDiscovery, s.handleDiscovery)
	s.RegisterHandler(MessageTypeDeviceInfo, s.handleDeviceInfo)
}

// 处理Ping消息
func (s *WebSocketServer) handlePing(client *WebSocketClient, msg WebSocketMessage) error {
	response := WebSocketMessage{
		Type:      MessageTypePong,
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"message": "pong",
		},
	}

	return s.SendToClient(client.ID, response)
}

// 处理连接消息
func (s *WebSocketServer) handleConnect(client *WebSocketClient, msg WebSocketMessage) error {
	// 处理设备连接信息
	deviceInfo := msg.Data["device_info"].(map[string]interface{})
	
	log.Printf("设备 %s 已连接: %s", client.ID, deviceInfo["device_name"])

	// 广播设备上线消息
	s.Broadcast(WebSocketMessage{
		Type:      MessageTypeDeviceInfo,
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"client_id":   client.ID,
			"device_info": deviceInfo,
			"action":      "online",
		},
	})

	return nil
}

// 处理设备发现消息
func (s *WebSocketServer) handleDiscovery(client *WebSocketClient, msg WebSocketMessage) error {
	// 获取在线设备列表并返回
	onlineDevices := s.getOnlineDevices()

	response := WebSocketMessage{
		Type:      MessageTypeDiscovery,
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"devices": onlineDevices,
		},
	}

	return s.SendToClient(client.ID, response)
}

// 处理设备信息消息
func (s *WebSocketServer) handleDeviceInfo(client *WebSocketClient, msg WebSocketMessage) error {
	// 更新设备信息
	// 可以在这里实现设备信息更新逻辑
	return nil
}

// 获取在线设备列表
func (s *WebSocketServer) getOnlineDevices() []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var devices []map[string]interface{}
	for _, client := range s.clients {
		devices = append(devices, map[string]interface{}{
			"client_id":   client.ID,
			"last_active": client.lastActive.Unix(),
			"status":      "online",
		})
	}

	return devices
}