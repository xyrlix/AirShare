## 项目概览
- 目标：跨 LAN/WAN 的去中心化文件/消息传输，P2P 首选，安全可验证，弱网稳健，支持断点续传与历史重发。
- 架构：前端 React/Vite PWA + WebRTC 数据通道；后端 Node.js/Express HTTPS + WebSocket 信令；LAN 发现 UDP 组播 + mDNS。
- 借鉴：Snapdrop（PWA/WebRTC/WebSocket/Node）、LocalSend（HTTPS 自签证书、REST API、离线优先）。

## 项目结构与规范
- 目录：`backend/`（HTTPS+WS+发现+文件API）/`frontend/`（PWA+UI+协议）/`README.md`。
- 语言/规范：TypeScript 全面启用；ESM 模块；统一错误处理与日志；MIT 许可；i18n（react‑i18next）。

## 后端实现
- HTTPS 服务：自签证书启动、暴露 `/api/files`（上传/分片/下载/管理）、`/api/cert` 指纹查询。
- WebSocket 信令 Hub：设备加入/离开、房间广播、点对点信令转发。
- 房间与令牌：`token/exp/sig`（HMAC256）签发与校验，10 分钟有效；防重放。
- LAN 发现：
  - mDNS：广播服务名、响应查询；
  - UDP 组播：心跳广播设备元信息，低依赖快速发现。
- 存储：本地文件数据卷（Docker 挂载）；临时上传缓存；清理策略。

## 前端实现
- P2P：双通道（控制/数据）WebRTC；ICE 状态监控与重试；候选收集与信令交换。
- UI 模块：设备列表、文件管理器、传输面板、历史记录、语言切换、通知。
- PWA：Service Worker（静态资源缓存、离线提示）、Manifest（图标/名称）、安装引导。
- i18n：中英等多语言切换，按路由/组件粒度加载文案。

## 传输协议（核心）
- 元信息：`meta(id,name,size,chunkSize,total,relativePath)` 双向握手；接收端返回已收分片索引。
- 分片：固定/自适应 `chunkSize`（默认 256KB）；顺序+缺片优先；发送方滑动窗口。
- ACK：每分片确认；更新速率与 ETA；记录时间戳用于 RTT。
- 断点续传：
  - 接收端：分片落盘 IndexedDB，刷新/重启后继续；完成后组装 Blob 并自动保存（可选目录）。
  - 发送端：以 `name|size|lastModified` 为指纹保存已确认集合；重选同文件自动续传。
- 回退：WebRTC 失败或超时自动切换 HTTP 分片上传，信令告知对端下载链接。

## 窗口自适应与弱网优化
- RTT 平滑：EWMA `srtt/rttvar` 估计；基于样本更新。
- Vegas 策略：实际吞吐 vs 期望吞吐比较调整窗口；抖动/丢包增大时减窗。
- 超时与重传：定时扫描未 ACK 分片，重传并指数降窗（最小 4）。
- CUBIC（可选）：在稳定网络下按时间函数平滑增窗，丢包时快速回退。

## 安全与隐私
- 端到端加密：WebRTC DTLS/SRTP 数据通道；HTTPS/TLS 保护 REST 与信令外层。
- 证书钉扎：二维码/URL 携带服务器指纹 `fp`，前端连接前校验。
- CORS 白名单：环境变量 `ALLOWED_ORIGINS` 控制；默认本机允许。
- 数据保护：不持久化敏感内容；上传目录与缓存独立；日志不含用户文件名。

## 部署与运维
- Docker 多阶段：安装依赖→构建前后端→运行层仅包含产物，`CMD node dist/index.js`。
- Compose：暴露 `8443`，挂载数据卷；可配置 `ROOM_SECRET/ALLOWED_ORIGINS/PORT`。
- 生产建议：反向代理（可选）、证书更新策略、资源限速/并发限制。

## 测试与验收
- 单元：分片索引/ACK/窗口调整/令牌签名校验。
- 集成：双端传输大文件（>1GB），中断/重连/刷新后续传；批量/目录与自动保存；HTTP 回退。
- 兼容：Chrome/Edge/Safari/iOS/Android；局域网热点与无路由环境；AP 隔离关闭提示。
- 指标：吞吐、RTT、丢包率、完成时延；弱网场景稳定性。

## 里程碑
- M1 基础后端/前端骨架、HTTPS/WS、文件管理、PWA。
- M2 P2P 传输与 ACK/窗口、HTTP 回退、历史记录。
- M3 断点续传（接收端 IndexedDB + 发送端指纹）、批量/目录与自动保存。
- M4 安全增强（令牌签名/证书钉扎/CORS 白名单）、LAN 发现（mDNS/UDP）。
- M5 弱网优化（RTT/Vegas/CUBIC/重传策略）、部署与完整测试。

## 交付物
- 完整代码库（frontend/backend/Dockerfile/docker-compose.yml）。
- 使用文档与快速启动（Docker 一键部署）。
- 测试报告与性能指标（典型场景与弱网对照）。

请确认以上计划；确认后我将开始编码与验证，并提供可访问的预览 URL。