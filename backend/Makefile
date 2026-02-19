.PHONY: build run test clean dev

# 构建项目
build:
	@echo "构建 AirShare 后端服务..."
	go build -o bin/airshare ./cmd/main.go

# 运行开发环境
run:
	@echo "启动 AirShare 开发服务器..."
	go run ./cmd/main.go -config config.yaml

# 运行测试
test:
	@echo "运行测试..."
	go test ./... -v

# 清理构建文件
clean:
	@echo "清理构建文件..."
	rm -rf bin/
	rm -rf storage/
	go clean

# 开发环境设置
dev: build
	@echo "启动开发环境..."
	./bin/airshare -config config.yaml

# Docker 构建
docker-build:
	@echo "构建 Docker 镜像..."
	docker build -t airshare-backend .

docker-run:
	@echo "运行 Docker 容器..."
	docker run -p 8080:8080 -v $(PWD)/storage:/app/storage airshare-backend

# 代码格式化
fmt:
	@echo "格式化代码..."
	go fmt ./...

# 依赖检查
dep-check:
	@echo "检查依赖..."
	go mod tidy
	go mod verify

# 性能分析
profile:
	@echo "启动性能分析..."
	go run ./cmd/main.go -config config.yaml -cpuprofile cpu.prof -memprofile mem.prof