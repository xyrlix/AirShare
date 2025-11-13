## 总览
- 目标：实现跨 LAN/WLAN 的去中心化、端到端加密、支持断点续传/重传/历史记录的文件传输与共享系统，覆盖 Web PWA、桌面与移动端。
- 模式：
  - LAN：mDNS/UDP 组播自动发现 + WebRTC P2P + 轻量 HTTP/WS，本地 TLS，自主可用，无外网依赖。
  - WLAN/跨网：房间号/二维码安全接入，WebSocket 信令，STUN/TURN 穿透，端到端加密。
- 参考对齐：借鉴 Snapdrop 的 PWA + WebRTC/WS 模式与 LocalSend 的 HTTPS 本地传输与 REST 协议思想，扩展为零外网依赖、断点续传与内容层加密。

## 架构设计
- 前端（PWA）：React + TypeScript + Vite；Zustand 管理状态；i18next 多语言；Service Worker 离线；WebRTC DataChannel + 回退 WebSocket/HTTP Range；IndexedDB 持久化历史与断点。
- 后端（Signaling/控制面）：Node.js + TypeScript（Fastify/Express）+ `ws`；JWT 鉴权；HTTPS；REST 文件管理与历史；与 TURN 协调。
- 局域网 Agent（可执行程序）：Go 或 Rust；mDNS（Bonjour/DNS‑SD）+ UDP 组播；轻量 HTTP/WS 服务；动态 TLS 证书；可选直接 WebRTC（桌面优化）。
- NAT 穿透：`coturn` 作为 TURN/STUN 服务（Docker 部署）。
- 存储：前端 IndexedDB；后端/Agent 使用 SQLite + 文件系统（元数据、分片清单、历史）。
- 安全：DTLS‑SRTP（WebRTC）+ 内容层 AES‑GCM（会话/文件密钥）；ECDH（X25519）握手派生；证书指纹与短码比对。

## 项目结构
AirShare/
- backend/
  - signaling/（WS 信令：房间、SDP、ICE、presence、密钥材料交换）
  - api/（REST：文件管理、历史、重发）
  - security/（ECDH/AES‑GCM、JWT、证书指纹校验）
  - storage/（SQLite/FS：文件元数据、传输记录、分片清单）
  - deploy/（Dockerfile、docker‑compose、coturn 配置）
- agent/
  - mdns/（注册 `_airshare._tcp`，TXT 携带设备名、端口、指纹）
  - multicast/（UDP 组播心跳/目录广播）
  - http/（上传/下载/Range/Chunk、有 TLS）
  - ws/（事件/控制面）
  - tls/（动态证书生成与指纹导出）
- frontend/
  - pages/（设备列表、传输队列、历史、设置、文件管理）
  - components/（拖拽上传、进度/速度仪表、通知、文件浏览器）
  - store/（设备/会话/队列/历史/设置）
  - network/（WebRTC/WS/HTTP 客户端、分片与回退）
  - i18n/（语言包）
  - sw/（Service Worker 与缓存策略）
  - pwa/（manifest、图标）

## 关键流程
- 设备发现：
  - LAN：Agent 注册 mDNS 服务；浏览器或桌面客户端通过本地网络查询发现设备；UDP 组播提供冗余心跳与目录聚合；展示设备列表。
  - WLAN：用户创建房间，生成二维码/短码；异网设备加入房间后通过 WS 信令交换 SDP/ICE；必要时走 TURN。
- 握手与密钥：
  - WS 通道内交换公钥（X25519）；派生会话密钥用于 AES‑GCM 内容层加密；展示证书指纹或短码用于人工比对。
- 传输：
  - 优先 WebRTC DataChannel（多路并发）；回退 WebSocket 二进制；再回退 HTTP Range（带 `Content‑Range`）。
  - 分片大小自适应（256KB‑2MB），基于 RTT/丢包调整并发（4‑8 并发）；ACK 机制记录已提交分片；断点续传通过 ChunkMap 恢复。
  - 批量队列：暂停/重试（指数退避）、优先级、速率限制（令牌桶）。
- 文件在线管理：远端目录浏览、上传/下载、删除、重命名、文件夹创建；可选单向/双向手动同步。
- 历史与重发：传输记录持久化；支持重发失败项；收件箱/发件箱视图。

## API/协议草案
- WebSocket 信令（`/ws`）：
  - `auth`（JWT）
  - `presence`（设备上线/下线）
  - `room.create`/`room.join`/`room.leave`
  - `offer`/`answer`/`candidate`
  - `key.exchange`（ECDH 公钥）
  - `session.confirm`（指纹/短码确认）
- REST：
  - `GET /files`（列表）
  - `POST /upload`（分片上传：`fileId`、`chunkIndex`、`totalChunks`、`hash`）
  - `GET /download/:id`（支持 Range）
  - `DELETE /files/:id`
  - `PATCH /files/:id`（重命名/移动）
  - `POST /folders`（创建）
  - `GET /history`、`POST /resend/:transferId`
- Agent 局域网协议：
  - mDNS TXT：`device`, `port`, `fp`
  - UDP 组播：`HELLO {deviceId, name, ts}`，`CATALOG {devices[]}`

## 数据模型
- Device：`id`, `name`, `addresses[]`, `fp`, `capabilities`
- FileMeta：`id`, `name`, `size`, `type`, `path`, `hash`
- ChunkMap：`fileId`, `chunkSize`, `count`, `ack[]`
- Transfer：`id`, `peerId`, `files[]`, `status`, `mode(LAN/WLAN)`, `progress`, `speed`, `startedAt`, `endedAt`, `error?`
- History：`transfers[]`
- Settings：`language`, `bandwidth`, `autoDiscover`, `security`

## 安全设计
- 端到端加密：WebRTC DTLS‑SRTP；此外对传输内容做 AES‑GCM 加密（每会话/每文件密钥），即使走 WS/HTTP 也保持 E2E。
- 身份与完整性：JWT 会话；证书指纹与房间短码比对；分片哈希与整体哈希校验；重放保护（nonce/IV 管理）。
- 证书：Agent 生成自签 TLS 证书（ECC），指纹对外暴露用于配对；后端站点用正式证书或本地开发证书。

## 前端体验
- 响应式布局，支持拖放与点击上传；清晰类型图标。
- 实时进度与速度；暂停/继续；批量与队列管理。
- 通知系统：浏览器通知 + 页内 Toast；错误与重试提示。
- 多语言：运行时切换，语言包 JSON。
- 离线：安装到桌面，断网任务排队，缓存最近设备与历史。

## 部署
- Docker Compose：
  - `web`：静态前端 + 反向代理 REST/WS
  - `signaling`：WS/REST 服务
  - `coturn`：TURN/STUN（端口与密钥配置）
- 单机模式：仅运行 Agent；前端访问 `http://localhost:<port>` 实现纯 LAN。

## 测试与验证
- 单元测试：
  - 前端（Vitest）：传输队列、分片逻辑、历史存储。
  - 后端（Jest）：API、信令、授权、安全。
  - Agent（Go/Rust）：mDNS/UDP/HTTP Range、TLS、ChunkMap 持久化。
- 集成/E2E：Playwright 双设备；弱网仿真（延迟/丢包/限速）；大文件（>2GB）断点续传；回退路径验证（WebRTC→WS→HTTP）。
- 安全测试：证书指纹比对流程、密钥派生正确性、GCM 认证失败处理。

## 里程碑
- M1（2–3 周）：
  - PWA 基础（框架、页面骨架、i18n、通知）
  - 后端信令与房间二维码/短码
  - WebRTC 直传（小文件），设备列表（WLAN）
  - 历史记录初版，Docker 基础
- M2（3–4 周）：
  - 分片并发、断点续传、回退到 WS/HTTP Range
  - 文件在线管理（列表/上传/下载/删除/重命名/文件夹）
  - 速率限制与弱网优化、错误重试
- M3（3–4 周）：
  - Agent（mDNS/UDP/TLS/HTTP Range/WS）
  - LAN 自动发现与纯离线模式、桌面打包
  - TURN 集成提升跨网成功率
- M4（2 周）：
  - 内容层加密、指纹/短码比对、安全加固
  - 同步模式与历史重发完善
  - Docker 一键部署完善与文档

## 风险与备选
- NAT 穿透失败：回退到 TURN；文件走 HTTP Range（E2E 加密）
- 浏览器限制：使用 Service Worker + Streams；桌面端通过 Agent 增强直连能力
- mDNS 在部分网络受限：UDP 组播与手动 IP/房间号混合发现
- 大文件性能：分片大小自适应、并发限速、零拷贝与流式读写

## 交付物
- 完整源码（frontend/backend/agent）
- Docker 镜像与 compose
- 多语言资源
- 测试套件与 E2E 脚本
- 使用指南与部署说明