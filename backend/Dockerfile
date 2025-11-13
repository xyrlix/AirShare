# 构建阶段
FROM golang:1.20-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o airshare ./cmd/main.go

# 运行阶段
FROM alpine:latest

# 安装必要的工具
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非root用户
RUN addgroup -S airshare && adduser -S airshare -G airshare

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/airshare .
COPY --from=builder /app/config.yaml .

# 创建存储目录
RUN mkdir -p storage && chown -R airshare:airshare /app

# 切换到非root用户
USER airshare

# 暴露端口
EXPOSE 8080

# 启动应用
CMD ["./airshare", "-config", "config.yaml"]