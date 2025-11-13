# AirShare - 跨平台文件传输管理系统

AirShare 是一个基于 P2P 技术的跨平台文件传输管理系统，专注于解决局域网和外网环境下的安全高效文件共享需求。

## 🌟 核心特性

### 🔍 智能设备发现
- **局域网自动发现**: 基于 mDNS 协议，零配置自动发现同网络设备
- **外网房间连接**: 通过二维码扫描或房间号建立安全连接
- **混合网络支持**: 同时支持 LAN 和 WLAN 网络环境

### 📁 多格式文件传输
- **文件类型**: 支持文件、文件夹、文本消息、剪贴板内容
- **批量传输**: 支持多文件/文件夹批量传输
- **断点续传**: 大文件传输中断后可继续传输

### 🛡️ 安全传输保障
- **端到端加密**: 所有传输内容加密保护
- **证书验证**: 设备间安全验证机制
- **离线传输**: 纯局域网传输，零外网依赖

### 📱 跨平台支持
- **Web 应用**: PWA 支持，离线可用
- **桌面端**: Windows、macOS、Linux
- **移动端**: Android、iOS

## 🏗️ 技术架构

### 前端技术栈
- **框架**: Flutter 3.0+
- **UI 组件**: Material Design 3
- **网络通信**: WebRTC、WebSocket、HTTP

### 后端技术栈  
- **语言**: Go 1.20+
- **网络协议**: mDNS、UDP组播、WebRTC DataChannel
- **安全加密**: TLS 1.3、端到端加密

## 📂 项目结构

```
AirShare/
├── backend/           # Go 后端服务
│   ├── cmd/          # 启动入口
│   ├── internal/     # 内部模块
│   │   ├── discovery/ # 设备发现
│   │   ├── transfer/  # 文件传输
│   │   ├── security/  # 安全模块
│   │   └── storage/   # 存储管理
│   └── pkg/          # 公共包
├── frontend/          # Flutter 前端
│   ├── lib/          # Dart 源代码
│   ├── assets/       # 静态资源
│   └── web/          # Web 构建输出
├── docs/             # 文档
└── scripts/          # 部署脚本
```

## 🚀 快速开始

### 环境要求
- Go 1.20+ 或 Flutter 3.0+
- Docker (可选，用于容器化部署)

### 开发运行
```bash
# 后端服务
cd backend
make run

# 前端应用  
cd frontend
flutter run
```

## 📖 使用说明

### 局域网传输
1. 在同一网络下启动 AirShare
2. 设备自动发现并显示在列表中
3. 选择设备，拖放文件开始传输

### 外网传输
1. 主设备创建房间并显示二维码
2. 其他设备扫描二维码或输入房间号
3. 建立安全连接后传输文件

## 🔧 配置选项

### 网络设置
- 传输端口配置
- 网络发现开关
- 加密级别设置

### 安全设置  
- 证书管理
- 连接密码
- 传输历史清理

## 🤝 参与贡献

欢迎提交 Issue 和 Pull Request 来改进 AirShare。

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## 🙏 致谢

感谢以下开源项目的启发：
- [Snapdrop](https://github.com/SnapDrop/snapdrop)
- [LocalSend](https://github.com/localsend/localsend)