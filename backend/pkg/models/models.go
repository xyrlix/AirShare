package models

import (
	"time"
)

// Device 是DeviceInfo的别名
type Device = DeviceInfo

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceStatusOnline  DeviceStatus = "online"
	DeviceStatusOffline DeviceStatus = "offline"
	DeviceStatusUnknown DeviceStatus = "unknown"
)

// DeviceType 设备类型
type DeviceType string

const (
	DeviceTypeUnknown  DeviceType = "unknown"
	DeviceTypeDesktop  DeviceType = "desktop"
	DeviceTypeMobile   DeviceType = "mobile"
	DeviceTypeWeb      DeviceType = "web"
	DeviceTypeTablet   DeviceType = "tablet"
)

// DeviceInfo 设备信息
type DeviceInfo struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Type      DeviceType   `json:"type"` // "desktop", "mobile", "web"
	OS        string       `json:"os"`
	Platform  string       `json:"platform"`
	IP        string       `json:"ip"`
	Port      int          `json:"port"`
	LastSeen  time.Time    `json:"last_seen"`
	IsOnline  bool         `json:"is_online"`
	Status    DeviceStatus `json:"status"`
	Version   string       `json:"version"`
}

// TransferRequest 传输请求
type TransferRequest struct {
	ID          string        `json:"id"`
	SenderID    string        `json:"sender_id"`
	ReceiverID  string        `json:"receiver_id"`
	Files       []FileInfo    `json:"files"`
	Status      TransferStatus `json:"status"`
	CreatedAt   time.Time     `json:"created_at"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	Error       string        `json:"error,omitempty"`
}

// FileInfo 文件信息
type FileInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	Type        string `json:"type"`
	Path        string `json:"path"`
	Checksum    string `json:"checksum"`
	ChunkSize   int    `json:"chunk_size"`
	TotalChunks int    `json:"total_chunks"`
	Progress    int    `json:"progress"` // 0-100
}

// TransferStatus 传输状态
type TransferStatus string

const (
	TransferPending   TransferStatus = "pending"
	TransferStarted   TransferStatus = "started"
	TransferProgress  TransferStatus = "progress"
	TransferCompleted TransferStatus = "completed"
	TransferFailed    TransferStatus = "failed"
	TransferCancelled TransferStatus = "cancelled"
)

// WebSocketMessage WebSocket消息
type WebSocketMessage struct {
	Type    string      `json:"type"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Target  string      `json:"target,omitempty"` // 目标设备ID
}

// MessageType 消息类型
const (
	MessageTypeDeviceList   = "device_list"
	MessageTypeDeviceUpdate = "device_update"
	MessageTypeTransfer     = "transfer"
	MessageTypeFileInfo     = "file_info"
	MessageTypeChunk        = "chunk"
	MessageTypeProgress     = "progress"
	MessageTypeError        = "error"
	MessageTypeKeepAlive    = "keep_alive"
)

// APIResponse API响应
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// ConnectRequest 连接请求
type ConnectRequest struct {
	RoomID string `json:"room_id"`
	Device DeviceInfo `json:"device"`
}

// RoomInfo 房间信息
type RoomInfo struct {
	ID        string      `json:"id"`
	HostID    string      `json:"host_id"`
	Devices   []DeviceInfo `json:"devices"`
	CreatedAt time.Time   `json:"created_at"`
	ExpiresAt time.Time   `json:"expires_at"`
}