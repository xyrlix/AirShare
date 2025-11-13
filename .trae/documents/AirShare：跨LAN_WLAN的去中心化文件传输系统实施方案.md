## 总览
- 目标：构建跨平台、跨网络（LAN/WLAN）、端到端加密、支持断点续传/历史/重传的去中心化文件传输系统。
- 模式：
  - LAN 去中心化模式：设备间自动发现（mDNS/UDP 组播），P2P 直连（WebRTC），本地轻量 HTTP/WS 服务，无外网依赖。
  - WLAN/跨网模式：安全接入（房间号/二维码），经信令与可选 TURN 中继穿透，端到端加密传输。
- 兼容：Web PWA、Windows/macOS/Linux 桌面、Android/iOS 移动；Docker 一键部署；多语言。

## 技术选型
- 前端（Web/PWA）：React + TypeScript + Vite；UI：Tailwind 或 Ant Design；状态：Zustand；i18n：i18next；文件 API：File System Access / Streams；WebRTC DataChannel；Service Worker 离线与缓存。
- 后端（服务组件）：
  - Signaling & 控制面：Node.js + TypeScript（Fastify/Express）+ `ws`；JWT 会话；HTTPS。
  - TURN/STUN：`coturn`（Docker）以提升 NAT 穿透成功率。
  - 去中心化设备代理（Agent）：Go 或 Rust 可执行，内置 mDNS（Bonjour/DNS-SD）、UDP 组播、轻量 HTTP/WS、TLS 动态证书（本地生成），跨平台打包。
- 传输层：优先 WebRTC DataChannel；回退 WebSocket + HTTP Range/Chunk；批量并发队列。
- 安全：
  - WebRTC 自带 DTLS-SRTP；额外内容层 AES‑GCM（每会话/每文件密钥）+ ECDH 握手派生；证书指纹/房间口令校验。
  - HTTPS/TLS：本地动态自签（Agent）或服务器证书；证书指纹比对。
- 存储：
  - 前端：IndexedDB（传输历史、断点清单、密钥材料）；
  - 后端/Agent：本地文件系统+SQLite（元数据、历史、分片清单）。
- 多语言：i18next JSON 资源，运行时动态切换。

## 模块划分
AirShare/
- backend/（TypeScript 服务）
  - `signaling/`：WebSocket 信令（设备上线/会话/候选/密钥材料通道）。
  - `api/`：REST（上传/下载/删除/重命名/列表/历史/重发）。
  - `security/`：ECDH+AES‑GCM、证书指纹校验、JWT。
  - `storage/`：SQLite/FS（清单、历史、断点）。
  - `discovery/`：WLAN 房间号/二维码；LAN 与 Agent 协同。
- agent/（Go 或 Rust）
  - `mdns/`：DNS‑SD 服务广播与发现（_airshare._tcp）。
  - `multicast/`：UDP 组播心跳与设备目录；
  - `http/`：轻量 HTTP（上传/下载/Range/Chunk）；
  - `ws/`：本地信令与事件；
  - `webrtc/`：可选（桌面直连传输优化）。
  - `tls/`：动态生成证书（含指纹暴露用于配对）。
- frontend/（PWA）
  - `pages/`：首页/设备列表/传输队列/历史/设置。
  - `components/`：拖拽上传、进度条、速度仪表、通知、文件浏览器。
  - `store/`：设备、会话、队列、历史、设置。
  - `network/`：WebRTC/WS/HTTP 客户端、分片上传、断点续传；
  - `i18n/`：多语言资源；
  - `sw/`：Service Worker，离线与缓存；
  - `pwa/`：manifest/安装；
- deploy/
  - `docker-compose.yml`：`signaling` + `web` + `coturn`；
  - `Dockerfile`：前端/后端镜像；

## 关键流程设计
- 设备发现：
  - LAN：Agent 在局域网广播服务（mDNS：`_airshare._tcp`），携带设备名/地址/证书指纹；浏览器前端与 Agent 通讯（同机或同网）获取设备目录；设备间直接建立连接。
  - WLAN：用户创建房间（后端 Signaling 分配），生成二维码/口令；异网设备扫码/输入加入，信令交换 SDP/ICE；必要时走 TURN。
- 密钥/安全：
  - 会话建立：ECDH（X25519）派生会话密钥；用于对文件内容层 AES‑GCM 加密（即使经 TURN/HTTP 也保持 E2E）。
  - 证书校验：展示/比对证书指纹或房间短码，防止中间人。
- 传输：
  - 分片与并发：默认 4–8 并发分片；分片大小自适应（256KB–2MB）。
  - 断点续传：分片清单与已确认偏移记录（前端 IndexedDB，后端/Agent SQLite+清单）；支持 `Content‑Range`/自定义 ACK。
  - 回退策略：WebRTC DataChannel → WebSocket → HTTP Range；弱网下自动降级并限速。
  - 批量：队列调度，优先级/暂停/重试指数退避。
- 文件在线管理：列表/上传/下载/删除/重命名/文件夹；远端目录映射；同步（单向/双向手动触发）。
- 历史/重发：持久化传输记录，支持重发；“发件箱/收件箱”视图。
- 通知与体验：浏览器通知、页面内通知、动画过渡；响应式布局；清晰类型图标。

## API/协议草案
- 信令（WS `/ws`）：
  - `auth`、`presence`（上线/下线）、`offer/answer/candidate`、`room.join`、`room.leave`、`session.key`（ECDH 公钥交换）。
- REST：
  - `GET /files`（列出）、`POST /upload`（分片上传）、`GET /download/:id`（Range 下载）、`DELETE /files/:id`、`PATCH /files/:id`（重命名）。
  - `GET /history`、`POST /resend/:id`。
- Agent 局域网协议：
  - mDNS TXT：`device`, `addr`, `port`, `fp`（证书指纹）。
  - UDP 心跳：`HELLO {deviceId, ts}`，设备目录广播。

## 数据模型
- Device：id、name、addresses、fp、capabilities。
- Transfer：id、peerId、fileId[]、status、progress、speed、startedAt/endedAt、mode（LAN/WLAN）。
- FileMeta：id、name、size、type、path、hash；ChunkMap：size、count、ack[]。
- History：transfers[]；
- Settings：language、bandwidth、auto‑discover、security。

## 安全与隐私
- 默认端到端加密；密钥仅存在双方设备（IndexedDB/本地安全存储）。
- 可选短码/指纹比对；所有连接强制 TLS（浏览器经 HTTPS，Agent 自签）。
- 不记录文件内容，仅记录元数据与历史；不上传云端；无广告无跟踪（MIT）。

## PWA/离线支持
- 安装到桌面；离线页面与最近历史可用；断网任务排队；热点模式支持本地 Agent。

## 部署
- Docker Compose：`web`（前端静态+API 反代）、`signaling`（WS/REST）、`coturn`（TURN），均网络隔离与端口暴露。
- 可选单机模式：仅启动 Agent，前端通过同机 `http://localhost` 访问。

## 测试与验证
- 单元测试：前端（Vitest）/后端（Jest）/Agent（Go/Rust 测试）。
- 集成/E2E：Playwright 跨两浏览器/两设备；弱网仿真（Chrome网络条件）。
- 传输正确性：大文件（>2GB）断点续传；错误注入（丢包/延迟）回退验证。

## 里程碑
- M1（2–3 周）：基础 PWA、信令服务、房间二维码/口令、WebRTC 直传、文件拖放、进度/速度、历史、i18n、Docker 基础。
- M2（3–4 周）：断点续传、分片并发与回退、文件在线管理（增删改查/重命名/文件夹）、通知系统、优化弱网与限速。
- M3（3–4 周）：Agent（mDNS/UDP/HTTPS 自签）、LAN 自动发现与无外网模式、桌面打包；TURN 集成、跨网稳定性增强。
- M4（2 周）：安全强化（证书指纹/短码比对、内容层加密）、同步模式、历史重发与收发箱完善、Docker 一键部署完善。

## 交付物
- 完整源码（frontend/backend/agent）与构建脚本；
- Docker Compose 与镜像；
- 多语言资源；
- 测试用例与E2E脚本；
- 使用指南与快速部署说明（README）。

## 与参考项目的对齐
- 借鉴 PWA+WebRTC/WebSocket 架构实现（无外部云依赖，局域网优先高速）。
- 局域网无外网依赖：本地生成 TLS 证书、REST+HTTPS、本地发现，提升在纯 LAN/热点环境的可用性。