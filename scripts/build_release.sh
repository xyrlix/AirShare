#!/bin/bash

# AirShare 发布构建脚本

echo "🚀 开始构建 AirShare 发布版本..."

# 检查是否在项目根目录
if [ ! -f "README.md" ]; then
    echo "❌ 请在项目根目录运行此脚本"
    exit 1
fi

# 创建构建目录
BUILD_DIR="build/release"
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

echo "📦 构建后端..."
cd backend

# 构建Linux版本
GOOS=linux GOARCH=amd64 go build -o "../$BUILD_DIR/airshare-linux-amd64" ./cmd
if [ $? -ne 0 ]; then
    echo "❌ Linux后端构建失败"
    exit 1
fi

# 构建Windows版本
GOOS=windows GOARCH=amd64 go build -o "../$BUILD_DIR/airshare-windows-amd64.exe" ./cmd
if [ $? -ne 0 ]; then
    echo "❌ Windows后端构建失败"
    exit 1
fi

# 构建macOS版本
GOOS=darwin GOARCH=amd64 go build -o "../$BUILD_DIR/airshare-darwin-amd64" ./cmd
if [ $? -ne 0 ]; then
    echo "❌ macOS后端构建失败"
    exit 1
fi

cd ..

echo "📱 构建前端..."
cd frontend

# 构建Web版本
flutter build web --release
if [ $? -ne 0 ]; then
    echo "❌ Web前端构建失败"
    exit 1
fi

# 复制Web构建文件
cp -r build/web "../$BUILD_DIR/web"

# 构建Android APK
flutter build apk --release
if [ $? -ne 0 ]; then
    echo "⚠️  Android APK构建失败，跳过"
else
    cp build/app/outputs/apk/release/app-release.apk "../$BUILD_DIR/airshare-android.apk"
fi

# 构建iOS应用
if [ "$(uname)" = "Darwin" ]; then
    flutter build ios --release
    if [ $? -ne 0 ]; then
        echo "⚠️  iOS构建失败，跳过"
    else
        cp -r build/ios/Release-iphoneos/Runner.app "../$BUILD_DIR/"
    fi
else
    echo "⚠️  非macOS系统，跳过iOS构建"
fi

cd ..

# 复制配置文件
echo "📁 复制配置文件..."
cp -r backend/config.yaml "$BUILD_DIR/"
cp docker-compose.yml "$BUILD_DIR/"
cp README.md "$BUILD_DIR/"

# 创建版本信息
VERSION=$(date +%Y%m%d-%H%M%S)
echo "AirShare Release $VERSION" > "$BUILD_DIR/VERSION"
echo "Build Date: $(date)" >> "$BUILD_DIR/VERSION"

# 创建部署脚本
echo "📜 创建部署脚本..."
cat > "$BUILD_DIR/deploy.sh" << 'EOF'
#!/bin/bash

echo "🚀 部署 AirShare..."

# 检查Docker
if ! command -v docker &> /dev/null; then
    echo "❌ Docker 未安装"
    exit 1
fi

# 检查Docker Compose
if ! command -v docker-compose &> /dev/null; then
    echo "❌ Docker Compose 未安装"
    exit 1
fi

# 启动服务
echo "📦 启动 AirShare 服务..."
docker-compose up -d

if [ $? -eq 0 ]; then
    echo "✅ AirShare 部署成功！"
    echo "🌐 访问地址: http://localhost:8080"
else
    echo "❌ AirShare 部署失败"
    exit 1
fi
EOF

chmod +x "$BUILD_DIR/deploy.sh"

# 创建压缩包
echo "🗜️ 创建发布包..."
cd "$BUILD_DIR"
tar -czf "../airshare-$VERSION.tar.gz" .
cd ../..

echo ""
echo "🎉 AirShare 发布版本构建完成！"
echo "📦 发布包位置: build/airshare-$VERSION.tar.gz"
echo ""
echo "📊 构建内容："
echo "- 后端可执行文件 (Linux/Windows/macOS)"
echo "- 前端Web应用"
echo "- Android APK"
if [ "$(uname)" = "Darwin" ]; then
    echo "- iOS应用"
fi
echo "- 配置文件"
echo "- 部署脚本"

# 显示文件大小
FILESIZE=$(du -h "build/airshare-$VERSION.tar.gz" | cut -f1)
echo "📏 发布包大小: $FILESIZE"

echo ""
echo "🚀 准备就绪，可以发布！"