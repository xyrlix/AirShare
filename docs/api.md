# AirShare API文档

## 概述

AirShare提供RESTful API用于文件传输和设备发现。所有API端点使用HTTPS安全传输。

## 基础信息

- **基础URL**: `https://localhost:8080/api/v1`
- **认证**: 无需认证，使用设备证书验证
- **响应格式**: JSON

## 设备发现API

### 获取设备列表

获取当前网络中可用的设备列表。

```http
GET /api/v1/devices
```

**响应示例**
```json
{
  "devices": [
    {
      "id": "device-123",
      "name": "My Laptop",
      "type": "desktop",
      "ip": "192.168.1.100",
      "port": 8080,
      "last_seen": "2024-01-01T10:00:00Z"
    }
  ]
}
```

## 文件传输API

### 发送文件

开始向目标设备发送文件。

```http
POST /api/v1/transfer/send
```

**请求体**
```json
{
  "target_device_id": "device-123",
  "file_path": "/path/to/file.txt",
  "file_name": "file.txt",
  "file_size": 1024,
  "file_hash": "sha256-hash"
}
```

**响应示例**
```json
{
  "transfer_id": "transfer-abc",
  "status": "started"
}
```

### 获取传输状态

获取指定传输的状态信息。

```http
GET /api/v1/transfer/{transfer_id}/status
```

**响应示例**
```json
{
  "transfer_id": "transfer-abc",
  "status": "in_progress",
  "progress": 50,
  "speed": "1.2 MB/s",
  "estimated_time": "00:01:30"
}
```

### 取消传输

取消指定的文件传输。

```http
POST /api/v1/transfer/{transfer_id}/cancel
```

**响应示例**
```json
{
  "status": "cancelled"
}
```

## 文件管理API

### 获取文件列表

获取当前设备上可用的文件列表。

```http
GET /api/v1/files
```

**响应示例**
```json
{
  "files": [
    {
      "name": "document.pdf",
      "size": 1024000,
      "type": "pdf",
      "modified": "2024-01-01T10:00:00Z"
    }
  ]
}
```

### 下载文件

下载指定文件。

```http
GET /api/v1/files/{filename}/download
```

**响应**: 文件二进制流

### 删除文件

删除指定文件。

```http
DELETE /api/v1/files/{filename}
```

**响应示例**
```json
{
  "status": "deleted"
}
```

## WebSocket API

### 实时通信

WebSocket端点用于实时通信和文件传输。

```http
GET /ws
```

**消息类型**
- `device_discovery`: 设备发现消息
- `transfer_request`: 传输请求
- `transfer_progress`: 传输进度
- `transfer_complete`: 传输完成
- `error`: 错误消息

## 错误处理

所有API在错误时返回标准错误格式：

```json
{
  "error": "Error message",
  "code": "ERROR_CODE"
}
```

**常见错误码**
- `DEVICE_NOT_FOUND`: 目标设备不存在
- `TRANSFER_NOT_FOUND`: 传输任务不存在
- `FILE_NOT_FOUND`: 文件不存在
- `PERMISSION_DENIED`: 权限不足

## 安全说明

- 所有通信使用TLS/SSL加密
- 设备间使用端到端加密
- 文件传输支持断点续传和校验
- 支持证书验证和身份验证