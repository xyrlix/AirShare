package transfer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// WebRTCTransferService 实现基于WebRTC的P2P文件传输
type WebRTCTransferService struct {
	mu         sync.RWMutex
	peers      map[string]*WebRTCPeer
	signalChan chan SignalMessage
	config     *WebRTCConfig
}

// WebRTCPeer 表示一个WebRTC对等连接
type WebRTCPeer struct {
	ID          string
	connection  *webrtc.PeerConnection
	datachannel *webrtc.DataChannel
	transfers   map[string]*FileTransfer
	onMessage   func(TransferMessage)
	onClose     func()
	mu          sync.RWMutex
}

// FileTransfer 表示文件传输任务
type FileTransfer struct {
	ID           string
	FileName     string
	FileSize     int64
	Progress     int64
	Status       TransferStatus
	Direction    TransferDirection
	PeerID       string
	StartTime    time.Time
	EndTime      time.Time
	Error        string
}

// TransferStatus 传输状态
type TransferStatus string

const (
	TransferPending    TransferStatus = "pending"
	TransferInProgress TransferStatus = "in_progress"
	TransferCompleted  TransferStatus = "completed"
	TransferFailed     TransferStatus = "failed"
	TransferCancelled  TransferStatus = "cancelled"
)

// TransferDirection 传输方向
type TransferDirection string

const (
	DirectionSend TransferDirection = "send"
	DirectionRecv TransferDirection = "receive"
)

// WebRTCConfig WebRTC配置
type WebRTCConfig struct {
	ICEServers []webrtc.ICEServer
	STUNServers []string
	TURNServers []string
	DataChannelConfig webrtc.DataChannelInit
}

// 监控指标
var (
	transferCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "airshare_transfers_total",
		Help: "Total number of file transfers",
	}, []string{"status", "direction"})
	
	transferBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "airshare_transfer_bytes_total",
		Help: "Total bytes transferred",
	}, []string{"direction"})
	
	activeConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "airshare_active_connections",
		Help: "Number of active WebRTC connections",
	})
)

// NewWebRTCTransferService 创建新的WebRTC传输服务
func NewWebRTCTransferService(config *WebRTCConfig) *WebRTCTransferService {
	return &WebRTCTransferService{
		peers:      make(map[string]*WebRTCPeer),
		signalChan: make(chan SignalMessage, 100),
		config:     config,
	}
}

// Start 启动WebRTC传输服务
func (s *WebRTCTransferService) Start(ctx context.Context) error {
	go s.signalProcessor(ctx)
	log.Println("WebRTC传输服务已启动")
	return nil
}

// CreateOffer 创建WebRTC offer
func (s *WebRTCTransferService) CreateOffer(peerID string) (*webrtc.SessionDescription, error) {
	config := webrtc.Configuration{
		ICEServers: s.config.ICEServers,
	}

	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, fmt.Errorf("创建对等连接失败: %v", err)
	}

	// 创建数据通道
	datachannel, err := peerConnection.CreateDataChannel("airshare", &s.config.DataChannelConfig)
	if err != nil {
		return nil, fmt.Errorf("创建数据通道失败: %v", err)
	}

	// 设置数据通道事件处理
	s.setupDataChannelHandlers(datachannel, peerID)

	// 设置ICE连接状态处理
	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		s.logConnectionState(peerID, state)
	})

	// 创建offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return nil, fmt.Errorf("创建offer失败: %v", err)
	}

	// 设置本地描述
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		return nil, fmt.Errorf("设置本地描述失败: %v", err)
	}

	// 保存对等连接
	peer := &WebRTCPeer{
		ID:          peerID,
		connection:  peerConnection,
		datachannel: datachannel,
		transfers:   make(map[string]*FileTransfer),
	}

	s.mu.Lock()
	s.peers[peerID] = peer
	s.mu.Unlock()

	activeConnections.Inc()

	return &offer, nil
}

// SetRemoteDescription 设置远程描述
func (s *WebRTCTransferService) SetRemoteDescription(peerID string, desc webrtc.SessionDescription) error {
	s.mu.RLock()
	peer, exists := s.peers[peerID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("对等连接不存在: %s", peerID)
	}

	return peer.connection.SetRemoteDescription(desc)
}

// AddICECandidate 添加ICE候选
func (s *WebRTCTransferService) AddICECandidate(peerID string, candidate webrtc.ICECandidateInit) error {
	s.mu.RLock()
	peer, exists := s.peers[peerID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("对等连接不存在: %s", peerID)
	}

	return peer.connection.AddICECandidate(candidate)
}

// SendFile 发送文件到指定的对等端
func (s *WebRTCTransferService) SendFile(peerID string, filePath string, metadata FileMetadata) error {
	s.mu.RLock()
	peer, exists := s.peers[peerID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("对等连接不存在: %s", peerID)
	}

	// 创建传输任务
	transferID := generateTransferID()
	transfer := &FileTransfer{
		ID:        transferID,
		FileName:  metadata.Name,
		FileSize:  metadata.Size,
		Status:    TransferPending,
		Direction: DirectionSend,
		PeerID:    peerID,
		StartTime: time.Now(),
	}

	peer.mu.Lock()
	peer.transfers[transferID] = transfer
	peer.mu.Unlock()

	// 发送文件元数据
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("序列化metadata失败: %v", err)
	}
	msg := TransferMessage{
		Type:        MessageTypeFileMetadata,
		TransferID:  transferID,
		Data:        metadataBytes,
		Timestamp:   time.Now().Unix(),
	}

	return s.sendMessage(peerID, msg)
}

// CancelTransfer 取消传输
func (s *WebRTCTransferService) CancelTransfer(peerID, transferID string) error {
	s.mu.RLock()
	peer, exists := s.peers[peerID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("对等连接不存在: %s", peerID)
	}

	peer.mu.Lock()
	defer peer.mu.Unlock()

	transfer, exists := peer.transfers[transferID]
	if !exists {
		return fmt.Errorf("传输任务不存在: %s", transferID)
	}

	if transfer.Status == TransferInProgress {
		transfer.Status = TransferCancelled
		transfer.EndTime = time.Now()
	
		// 发送取消消息
		msg := TransferMessage{
			Type:       MessageTypeCancelTransfer,
			TransferID: transferID,
		}
		return s.sendMessage(peerID, msg)
	}

	return nil
}

// 设置数据通道事件处理
func (s *WebRTCTransferService) setupDataChannelHandlers(dc *webrtc.DataChannel, peerID string) {
	dc.OnOpen(func() {
		log.Printf("数据通道已打开: %s", peerID)
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		var transferMsg TransferMessage
		if err := json.Unmarshal(msg.Data, &transferMsg); err != nil {
			log.Printf("解析消息失败: %v", err)
			return
		}

		s.handleMessage(peerID, transferMsg)
	})

	dc.OnClose(func() {
		log.Printf("数据通道已关闭: %s", peerID)
		s.closePeer(peerID)
	})
}

// 处理接收到的消息
func (s *WebRTCTransferService) handleMessage(peerID string, msg TransferMessage) {
	switch msg.Type {
	case MessageTypeFileMetadata:
		s.handleFileMetadata(peerID, msg)
	case MessageTypeFileChunk:
		s.handleFileChunk(peerID, msg)
	case MessageTypeTransferComplete:
		s.handleTransferComplete(peerID, msg)
	case MessageTypeCancelTransfer:
		s.handleCancelTransfer(peerID, msg)
	default:
		log.Printf("未知消息类型: %s", msg.Type)
	}
}

// 发送消息
func (s *WebRTCTransferService) sendMessage(peerID string, msg TransferMessage) error {
	s.mu.RLock()
	peer, exists := s.peers[peerID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("对等连接不存在: %s", peerID)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return peer.datachannel.Send(data)
}

// 记录连接状态
func (s *WebRTCTransferService) logConnectionState(peerID string, state webrtc.PeerConnectionState) {
	log.Printf("对等连接 %s 状态: %s", peerID, state.String())
	
	switch state {
	case webrtc.PeerConnectionStateConnected:
		log.Printf("WebRTC连接已建立: %s", peerID)
	case webrtc.PeerConnectionStateDisconnected:
		log.Printf("WebRTC连接已断开: %s", peerID)
	case webrtc.PeerConnectionStateFailed:
		log.Printf("WebRTC连接失败: %s", peerID)
		s.closePeer(peerID)
	case webrtc.PeerConnectionStateClosed:
		log.Printf("WebRTC连接已关闭: %s", peerID)
		s.closePeer(peerID)
	}
}

// 关闭对等连接
func (s *WebRTCTransferService) closePeer(peerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if peer, exists := s.peers[peerID]; exists {
		if peer.connection != nil {
			peer.connection.Close()
		}
		delete(s.peers, peerID)
		activeConnections.Dec()
		log.Printf("已关闭对等连接: %s", peerID)
	}
}

// 信号处理器
func (s *WebRTCTransferService) signalProcessor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-s.signalChan:
			s.processSignalMessage(msg)
		}
	}
}

// 处理信号消息
func (s *WebRTCTransferService) processSignalMessage(msg SignalMessage) {
	// 实现信号交换逻辑
	// 这里可以集成NAT穿透、STUN/TURN服务等
}

// 生成传输ID
func generateTransferID() string {
	return fmt.Sprintf("transfer_%d", time.Now().UnixNano())
}

// 辅助方法：处理文件元数据、文件分片等（具体实现略）
func (s *WebRTCTransferService) handleFileMetadata(peerID string, msg TransferMessage) {
	// 实现文件元数据处理逻辑
}

func (s *WebRTCTransferService) handleFileChunk(peerID string, msg TransferMessage) {
	// 实现文件分片处理逻辑
}

func (s *WebRTCTransferService) handleTransferComplete(peerID string, msg TransferMessage) {
	// 实现传输完成处理逻辑
}

func (s *WebRTCTransferService) handleCancelTransfer(peerID string, msg TransferMessage) {
	// 实现取消传输处理逻辑
}