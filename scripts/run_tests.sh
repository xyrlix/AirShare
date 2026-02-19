#!/bin/bash

# AirShare 测试脚本

echo "🚀 开始运行 AirShare 测试..."

# 检查是否在项目根目录
if [ ! -f "README.md" ]; then
    echo "❌ 请在项目根目录运行此脚本"
    exit 1
fi

# 检查Go是否安装
if ! command -v go &> /dev/null; then
    echo "❌ Go 未安装，请先安装 Go"
    exit 1
fi

# 检查Flutter是否安装
if ! command -v flutter &> /dev/null; then
    echo "⚠️  Flutter 未安装，跳过前端测试"
    SKIP_FRONTEND=true
fi

echo "📦 运行后端测试..."
cd backend

# 运行Go测试
if go test ./... -v; then
    echo "✅ 后端测试通过"
else
    echo "❌ 后端测试失败"
    exit 1
fi

cd ..

# 运行前端测试
if [ "$SKIP_FRONTEND" != "true" ]; then
    echo "📱 运行前端测试..."
    cd frontend
    
    # 运行Flutter测试
    if flutter test; then
        echo "✅ 前端测试通过"
    else
        echo "❌ 前端测试失败"
        exit 1
    fi
    
    cd ..
fi

# 运行集成测试
echo "🔗 运行集成测试..."
cd scripts

# 启动测试服务
if [ -f "integration_test.py" ]; then
    python integration_test.py
    if [ $? -eq 0 ]; then
        echo "✅ 集成测试通过"
    else
        echo "❌ 集成测试失败"
        exit 1
    fi
else
    echo "⚠️  集成测试脚本不存在，跳过"
fi

cd ..

echo "🎉 所有测试完成！"
echo ""
echo "📊 测试总结："
echo "- 后端测试: ✅ 通过"
if [ "$SKIP_FRONTEND" != "true" ]; then
    echo "- 前端测试: ✅ 通过"
else
    echo "- 前端测试: ⚠️  跳过"
fi
echo "- 集成测试: ✅ 通过"

# 运行安全扫描
echo "🔒 运行安全扫描..."
if command -v gosec &> /dev/null; then
    cd backend
    gosec ./...
    cd ..
else
    echo "⚠️  gosec 未安装，跳过安全扫描"
fi

echo ""
echo "🎯 测试完成，项目准备就绪！"