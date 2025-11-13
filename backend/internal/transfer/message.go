package transfer

import (
	"encoding/json"
	"fmt"
	"time"
)

// 消息类型枚举
const (
	MessageTypeFileMetadata    = "file_metadata"
	MessageTypeFileChunk       = "file_chunk"
	MessageTypeTransferComplete = "transfer_complete"
	MessageTypeCancelTransfer  = "cancel_transfer"
	MessageTypeError           = "error"
	MessageTypePing            = "ping"
	MessageTypePong            = "pong"
)

// TransferMessage 传输消息结构
type TransferMessage struct {
	Type        string          `json:"type"`         // 消息类型
	TransferID  string          `json:"transfer_id"`  // 传输ID
	Data        json.RawMessage `json:"data"`         // 消息数据
	Timestamp   int64           `json:"timestamp"`   // 时间戳
	Sequence    int             `json:"sequence"`     // 序列号
	Checksum    string          `json:"checksum"`     // 校验和
}

// FileMetadata 文件元数据
type FileMetadata struct {
	Name     string `json:"name"`     // 文件名
	Size     int64  `json:"size"`     // 文件大小
	Type     string `json:"type"`     // 文件类型
	Checksum string `json:"checksum"` // 文件校验和
	Chunks   int    `json:"chunks"`   // 分片数量
	ChunkSize int64 `json:"chunk_size"` // 分片大小
}

// FileChunk 文件分片数据
type FileChunk struct {
	Index    int    `json:"index"`    // 分片索引
	Offset   int64  `json:"offset"`   // 文件偏移量
	Size     int64  `json:"size"`     // 分片大小
	Data     []byte `json:"data"`     // 分片数据
	Checksum string `json:"checksum"` // 分片校验和
	IsLast   bool   `json:"is_last"`  // 是否为最后一个分片
}

// TransferComplete 传输完成消息
type TransferComplete struct {
	Success    bool   `json:"success"`    // 是否成功
	Error      string `json:"error"`      // 错误信息
	TotalTime  int64  `json:"total_time"` // 总耗时（毫秒）
	AverageSpeed float64 `json:"average_speed"` // 平均速度（字节/秒）
}

// ErrorMessage 错误消息
type ErrorMessage struct {
	Code    string `json:"code"`    // 错误代码
	Message string `json:"message"` // 错误信息
	Details string `json:"details"` // 详细错误信息
}

// SignalMessage 信号消息（用于WebRTC信令交换）
type SignalMessage struct {
	Type      string `json:"type"`      // 信号类型：offer, answer, candidate
	SDP       string `json:"sdp"`       // SDP信息
	Candidate string `json:"candidate"` // ICE候选信息
	Target    string `json:"target"`    // 目标设备ID
	Source    string `json:"source"`    // 源设备ID
}

// NewTransferMessage 创建新的传输消息
func NewTransferMessage(msgType, transferID string, data interface{}) (*TransferMessage, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("序列化消息数据失败: %v", err)
	}

	// 计算校验和
	checksum := calculateChecksum(jsonData)

	return &TransferMessage{
		Type:       msgType,
		TransferID: transferID,
		Data:       jsonData,
		Timestamp:  time.Now().UnixMilli(),
		Checksum:   checksum,
	}, nil
}

// ParseMessageData 解析消息数据
func (msg *TransferMessage) ParseMessageData(target interface{}) error {
	if err := json.Unmarshal(msg.Data, target); err != nil {
		return fmt.Errorf("解析消息数据失败: %v", err)
	}
	return nil
}

// ValidateChecksum 验证消息校验和
func (msg *TransferMessage) ValidateChecksum() bool {
	expectedChecksum := calculateChecksum(msg.Data)
	return msg.Checksum == expectedChecksum
}

// Serialize 序列化消息
func (msg *TransferMessage) Serialize() ([]byte, error) {
	return json.Marshal(msg)
}

// DeserializeMessage 反序列化消息
func DeserializeMessage(data []byte) (*TransferMessage, error) {
	var msg TransferMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("反序列化消息失败: %v", err)
	}
	return &msg, nil
}

// CreateFileMetadataMessage 创建文件元数据消息
func CreateFileMetadataMessage(transferID string, metadata *FileMetadata) (*TransferMessage, error) {
	return NewTransferMessage(MessageTypeFileMetadata, transferID, metadata)
}

// CreateFileChunkMessage 创建文件分片消息
func CreateFileChunkMessage(transferID string, chunk *FileChunk) (*TransferMessage, error) {
	return NewTransferMessage(MessageTypeFileChunk, transferID, chunk)
}

// CreateTransferCompleteMessage 创建传输完成消息
func CreateTransferCompleteMessage(transferID string, complete *TransferComplete) (*TransferMessage, error) {
	return NewTransferMessage(MessageTypeTransferComplete, transferID, complete)
}

// CreateCancelTransferMessage 创建取消传输消息
func CreateCancelTransferMessage(transferID string) (*TransferMessage, error) {
	return NewTransferMessage(MessageTypeCancelTransfer, transferID, nil)
}

// CreateErrorMessage 创建错误消息
func CreateErrorMessage(transferID string, errorMsg *ErrorMessage) (*TransferMessage, error) {
	return NewTransferMessage(MessageTypeError, transferID, errorMsg)
}

// CreatePingMessage 创建Ping消息
func CreatePingMessage() (*TransferMessage, error) {
	return NewTransferMessage(MessageTypePing, "", map[string]interface{}{
		"timestamp": time.Now().UnixMilli(),
	})
}

// CreatePongMessage 创建Pong消息
func CreatePongMessage() (*TransferMessage, error) {
	return NewTransferMessage(MessageTypePong, "", map[string]interface{}{
		"timestamp": time.Now().UnixMilli(),
	})
}

// 计算校验和（简化实现，实际应该使用更安全的哈希算法）
func calculateChecksum(data []byte) string {
	// 这里使用简单的累加和作为校验和
	// 在实际应用中应该使用SHA256等安全哈希算法
	sum := 0
	for _, b := range data {
		sum += int(b)
	}
	return fmt.Sprintf("%08x", sum)
}

// MessageHandler 消息处理器接口
type MessageHandler interface {
	HandleMessage(msg *TransferMessage) error
}

// MessageRouter 消息路由器
type MessageRouter struct {
	handlers map[string]MessageHandler
}

// NewMessageRouter 创建新的消息路由器
func NewMessageRouter() *MessageRouter {
	return &MessageRouter{
		handlers: make(map[string]MessageHandler),
	}
}

// RegisterHandler 注册消息处理器
func (r *MessageRouter) RegisterHandler(msgType string, handler MessageHandler) {
	r.handlers[msgType] = handler
}

// RouteMessage 路由消息
func (r *MessageRouter) RouteMessage(msg *TransferMessage) error {
	handler, exists := r.handlers[msg.Type]
	if !exists {
		return fmt.Errorf("未知的消息类型: %s", msg.Type)
	}

	// 验证校验和
	if !msg.ValidateChecksum() {
		return fmt.Errorf("消息校验和验证失败")
	}

	return handler.HandleMessage(msg)
}

// BroadcastMessage 广播消息（用于多播传输）
type BroadcastMessage struct {
	Message   *TransferMessage `json:"message"`
	Targets   []string         `json:"targets"`  // 目标设备列表
	Priority  int              `json:"priority"` // 优先级
	RetryCount int             `json:"retry_count"` // 重试次数
}

// CreateBroadcastMessage 创建广播消息
func CreateBroadcastMessage(msg *TransferMessage, targets []string) *BroadcastMessage {
	return &BroadcastMessage{
		Message:   msg,
		Targets:   targets,
		Priority:  1,
		RetryCount: 0,
	}
}