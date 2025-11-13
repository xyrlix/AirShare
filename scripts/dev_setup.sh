#!/bin/bash

# AirShare 开发环境设置脚本

echo "🚀 AirShare 开发环境设置脚本"
echo "================================"

# 检查依赖
check_dependencies() {
    echo "🔍 检查系统依赖..."
    
    # 检查Go
    if command -v go &> /dev/null; then
        echo "✅ Go 已安装: $(go version)"
    else
        echo "❌ Go 未安装，请先安装Go"
        exit 1
    fi
    
    # 检查Flutter
    if command -v flutter &> /dev/null; then
        echo "✅ Flutter 已安装: $(flutter --version | head -1)"
    else
        echo "⚠️  Flutter 未安装，仅能运行后端服务"
    fi
    
    # 检查Docker
    if command -v docker &> /dev/null; then
        echo "✅ Docker 已安装"
    else
        echo "⚠️  Docker 未安装，无法使用容器化部署"
    fi
}

# 设置后端开发环境
setup_backend() {
    echo "📦 设置后端环境..."
    cd backend
    
    # 下载依赖
    echo "📥 下载Go依赖..."
    go mod download
    
    # 构建应用
    echo "🔨 构建后端应用..."
    go build -o bin/airshare ./cmd/main.go
    
    # 创建存储目录
    mkdir -p storage
    
    cd ..
    echo "✅ 后端环境设置完成"
}

# 设置前端开发环境
setup_frontend() {
    if command -v flutter &> /dev/null; then
        echo "📦 设置前端环境..."
        cd frontend
        
        # 下载依赖
        echo "📥 下载Flutter依赖..."
        flutter pub get
        
        # 创建资源目录
        mkdir -p assets/images assets/animations assets/translations
        
        cd ..
        echo "✅ 前端环境设置完成"
    else
        echo "⏭️  跳过前端环境设置"
    fi
}

# 显示使用说明
show_usage() {
    echo ""
    echo "📖 使用说明:"
    echo ""
    echo "启动后端服务:"
    echo "  cd backend && go run ./cmd/main.go"
    echo ""
    echo "启动前端应用:"
    echo "  cd frontend && flutter run"
    echo ""
    echo "构建Docker镜像:"
    echo "  docker-compose build"
    echo ""
    echo "启动完整服务:"
    echo "  docker-compose up -d"
    echo ""
}

# 主函数
main() {
    check_dependencies
    setup_backend
    setup_frontend
    show_usage
    
    echo "🎉 环境设置完成！"
    echo ""
    echo "快速开始:"
    echo "  1. 启动后端: cd backend && make run"
    echo "  2. 访问 http://localhost:8080"
    echo ""
}

# 运行主函数
main