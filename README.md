## 项目概览
- 目标：跨 LAN/WAN 的去中心化文件/消息传输，P2P 首选，安全可验证，弱网稳健，支持断点续传与历史重发。
- 架构：前端 React/Vite PWA + WebRTC 数据通道；后端 Node.js/Express HTTPS + WebSocket 信令；LAN 发现 UDP 组播 + mDNS。
- 借鉴：Snapdrop（PWA/WebRTC/WebSocket/Node）、LocalSend（HTTPS 自签证书、REST API、离线优先）。

## 项目功能
- 设备发现：后端信令房间与 mDNS/UDP 发现（Agent）。
- P2P 传输：WebRTC `ctrl/data/text` 三通道，分片 ACK 与滑动窗口。
- HTTP 回退：P2P不可用时自动启用 `/api/files` 分片上传与 Range 下载。
- 文件管理：列表/上传/下载/删除/重命名（前端内置 FileManager，对接后端与 Agent）。
- 断点续传：接收端 IndexedDB 持久化，发送端按文件指纹保存已确认集合。
- 离线缓存：Service Worker 缓存静态与 `/api/files` GET；非 GET 请求入队重试。
- 会话协商：`POST /api/session/key` 透传 `version: 1`；`GET /api/session/info` 返回 `{ version, ecdh, cipher, iv }`；前端 ECDH 派生 AES‑GCM。
- UI/体验：Ant Design 集成，进度条与通知，局域网设备列表。
- 加密开关：`Settings.encrypt` 控制是否加密分片，支持明文/加密双模式。

## 流程逻辑
- 房间与令牌
  - 前端进入页面后获取或生成房间号，向后端请求签名令牌 `token/exp/sig`（`backend/src/index.ts:33`）。
  - 通过 WebSocket 加入房间；Hub 验证签名后允许信令转发（`backend/src/signaling/hub.ts:20`）。
  - 通过 `/api/qr/room` 生成包含 `fp` 的二维码便于多设备加入（`backend/src/api/qr.ts:6`）。
- 证书钉扎
  - 前端在启动时拉取 `/api/cert` 指纹并与 URL 携带的 `fp` 比较，不一致则阻断（`backend/src/index.ts:40`）。
- 会话密钥
  - 主叫在建立连接时生成密钥对并上报 `/api/session/key`（携带 `version: 1`），同时通过信令发送公钥给对端（`frontend/src/App.tsx:135`）。
  - 被叫侧收到 `signal:key` 时若不存在密钥对会即时生成并回应，携带 `v: 1`；随后双端完成 ECDH，派生 AES‑GCM 会话密钥（`frontend/src/App.tsx:126–134`）。
  - 能力查询：`GET /api/session/info` 返回 `{ version: 1, ecdh: 'P-256', cipher: 'AES-GCM', iv: 'salt:index derived 12 bytes' }`（`backend/src/api/session.ts:9–12`）。
- WebRTC 连接
  - 创建 `RTCPeerConnection`，建立 `ctrl/data/text` 三通道；候选收集与信令交换（`frontend/src/App.tsx:64`）。
  - `data` 通道承载分片数据，`ctrl` 承载元信息与 ACK，`text` 承载纯文本。
  - WebSocket 信令事件采用 `addEventListener('message')` 统一分发，避免 `onmessage` 覆盖（`frontend/src/App.tsx:121–134`、`frontend/src/App.tsx:399–406`）。
- 文件传输
  - 发送端：发 `meta`（名称/大小/分片数/盐），根据接收端返回状态集合计算 `missing` 并滑动窗口发送（`frontend/src/App.tsx:141`）。
  - ACK：每分片确认后更新吞吐与 ETA，依据 `srtt/rttvar` 与期望吞吐动态调整窗口（`frontend/src/App.tsx:93`）。
  - 超时与重传：定时扫描未确认分片，指数回退与退避，直到全部确认或回退 HTTP（`frontend/src/App.tsx:248`）。
  - 回退 HTTP：按分片 POST `/api/files/upload/*`，完成后信令告知下载链接；接收端 Range 下载并解密组装（`frontend/src/App.tsx:180, 206`）。
- 接收持久化
  - 接收端分片入库 IndexedDB，页面刷新后可继续；完成后组装 Blob 并记录历史（`frontend/src/services/db.ts`）。
- 离线与重试
  - Service Worker 注册与缓存（`frontend/src/main.tsx:6`），导航离线回退页面；`/api/files` GET 缓存；非 GET 入队指数退避重试（`frontend/public/sw.js:107`）。
  - 队列可视化与去重：SW 会广播 `{ type: 'queue-stats', size }` 队列大小到页面；离线入队按 `method:url:bodyLength` 去重，防重复提交（`frontend/public/sw.js:148–186`）。
  - 页面查询队列示例：`navigator.serviceWorker.controller?.postMessage({ type: 'get-queue' })`；监听：`navigator.serviceWorker.addEventListener('message', e => { if (e.data?.type==='queue-stats') console.log(e.data.size) })`。

## 部署与运行
- 前提环境
  - Node.js 与 npm：前端构建与后端开发运行所需（Vite 与 TypeScript）。
  - Go 1.20+：可选（Agent 文件服务与 LAN 发现）。
  - Windows/macOS/Linux 均可；浏览器需允许自签证书。
- 开发运行（推荐单进程）
  - 后端：`cd backend && npm run dev`（HTTPS+WS 启动在 `8443`）。
  - 前端：使用后端静态托管以保证同源，先构建：`cd frontend && npm ci && npm run build`；后端会在存在 `frontend/dist` 时自动挂载根路径（`backend/src/index.ts:43`）。
  - 访问：`https://localhost:8443/`。
- 打包分发
  - 安装 `pkg`: `npm i -g pkg`。
  - 运行脚本：`pwsh ./deploy/pkg.ps1`。
    - 后端：编译为可执行文件到 `dist/airshare`。
    - 前端：若检测到 `npm`，自动构建并复制到 `dist/frontend/dist`；若缺失 `npm`，脚本将跳过构建但继续后端打包（`deploy/pkg.ps1:1`）。
- Agent（可选，纯 LAN/热点场景）
  - 运行：`go run ./agent/main.go`（HTTP `:8444`，HTTPS `:8443`）。
  - 前端 LAN 设备列表可显示 mDNS 发现的设备并跳转（`frontend/src/components/LanDevices.tsx:7`）。
  - 注意端口避免与后端冲突，建议不同设备或修改端口。

## 注意事项
- 同源要求：前端 API 基于 `window.location.origin`，生产建议通过后端托管静态资源，避免 dev server 不同源导致请求指向 Vite 端口。
- 端口冲突：Agent 与后端默认均使用 `8443` HTTPS，需分机部署或调整端口以避免冲突。
- 自签证书：首次访问浏览器会有警告；生产环境建议配置可信证书或浏览器信任策略。
- 指纹校验：URL 携带 `fp` 与 `/api/cert` 返回一致才允许连接，确保防劫持。
- CORS：`ALLOWED_ORIGINS` 环境变量控制允许来源；跨域开发需正确设置。
- 数据目录：后端存储在 `backend/data`，Agent 存储在当前工作目录 `./data`；注意持久化与权限。
- 离线重试：Service Worker 对非 GET 的 `/api/files` 请求进行入队重试；避免重复操作导致多次提交，前端交互应提示队列状态。
- 大文件与性能：默认分片 `256KB`；弱网下窗口会降低保障稳定性，必要时可在设置中限速。
- 依赖清理：后端移除未使用的 `jsonwebtoken` 依赖（`backend/package.json:18–19`）。

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