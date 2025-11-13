# AirShare 开发文档

## 项目概述

AirShare 是一个基于 P2P 技术的跨平台文件传输管理系统，专注于解决局域网和外网环境下的安全高效文件共享需求。

## 技术架构

### 后端技术栈
- **语言**: Go 1.20+
- **网络通信**: HTTP/HTTPS, WebSocket, mDNS
- **文件传输**: 分片传输、断点续传
- **数据存储**: 内存 + 文件系统
- **服务发现**: mDNS (Bonjour/Zeroconf)

### 前端技术栈
- **框架**: Flutter 3.0+
- **UI组件**: Material Design 3
- **状态管理**: Riverpod
- **路由**: Go Router
- **网络**: WebSocket, HTTP

## 快速开始

### 环境要求
- Go 1.20+ 或 Flutter 3.0+
- Docker (可选)

### 开发环境设置

```bash
# 克隆项目
git clone <repository-url>
cd AirShare

# 设置开发环境
chmod +x scripts/dev_setup.sh
./scripts/dev_setup.sh

# 启动后端服务
cd backend
make run

# 启动前端应用 (需要Flutter环境)
cd frontend
flutter run
```

## 项目结构

### 后端结构
```
backend/
├── cmd/main.go              # 入口文件
├── internal/
│   ├── config/             # 配置管理
│   ├── discovery/          # 设备发现服务
│   ├── transfer/           # 文件传输服务
│   └── server/            # HTTP/WebSocket服务器
├── pkg/models/            # 数据模型
└── config.yaml            # 配置文件
```

### 前端结构
```
frontend/
├── lib/
│   ├── main.dart           # 入口文件
│   ├── app.dart           # 应用配置
│   ├── config/            # 配置管理
│   ├── common/            # 公共组件
│   └── features/          # 功能模块
└── pubspec.yaml           # 依赖配置
```

## 核心功能模块

### 1. 设备发现模块

**功能**: 局域网设备自动发现和连接管理

**实现原理**:
- 使用 mDNS (Multicast DNS) 协议进行设备发现
- 定期广播设备信息和查询其他设备
- 维护设备状态和在线状态

**关键文件**:
- `backend/internal/discovery/service.go`
- `frontend/lib/features/device_discovery/`

### 2. 文件传输模块

**功能**: 文件分片传输、断点续传、进度监控

**实现原理**:
- 文件分片传输 (默认64KB分片)
- SHA256 校验和验证
- 断点续传支持
- 传输状态实时更新

**关键文件**:
- `backend/internal/transfer/service.go`
- `frontend/lib/features/file_transfer/`

### 3. WebSocket通信模块

**功能**: 实时通信和设备状态同步

**消息类型**:
- 设备列表更新
- 传输状态更新
- 文件分片传输
- 心跳检测

**关键文件**:
- `backend/internal/server/server.go`
- `frontend/lib/services/websocket_service.dart`

## API 接口

### REST API

#### 设备管理
- `GET /api/devices` - 获取设备列表
- `POST /api/devices/connect` - 连接设备

#### 文件传输
- `POST /api/transfer` - 开始传输
- `GET /api/transfer/:id` - 获取传输状态
- `PUT /api/transfer/:id/cancel` - 取消传输

### WebSocket API

连接地址: `ws://localhost:8080/ws`

消息格式:
```json
{
  "type": "device_list|transfer|progress|error",
  "data": {},
  "error": "",
  "target": "device_id"
}
```

## 部署指南

### Docker 部署

```bash
# 构建镜像
docker-compose build

# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f
```

### 生产环境配置

1. **修改配置文件** `backend/config.yaml`
2. **设置 TLS 证书**
3. **配置防火墙规则**
4. **设置反向代理** (可选)

## 开发指南

### 添加新功能

1. **后端开发**
   - 在 `pkg/models/` 添加数据模型
   - 在 `internal/` 创建服务模块
   - 在 `internal/server/` 添加API接口

2. **前端开发**
   - 在 `lib/features/` 创建功能模块
   - 添加状态管理 Provider
   - 更新路由配置

### 测试

```bash
# 后端测试
cd backend
go test ./... -v

# 前端测试  
cd frontend
flutter test
```

### 性能优化

1. **文件传输优化**
   - 调整分片大小
   - 并发传输控制
   - 内存使用优化

2. **网络通信优化**
   - WebSocket 连接复用
   - 消息压缩
   - 心跳机制优化

## 故障排除

### 常见问题

1. **设备无法发现**
   - 检查防火墙设置
   - 验证 mDNS 服务运行状态
   - 检查网络配置

2. **文件传输失败**
   - 检查文件大小限制
   - 验证存储空间
   - 检查网络连接

3. **WebSocket连接断开**
   - 检查心跳机制
   - 验证网络稳定性
   - 调整超时设置

## 贡献指南

1. Fork 项目
2. 创建功能分支
3. 提交代码变更
4. 创建 Pull Request

## 许可证

本项目采用 MIT 许可证。