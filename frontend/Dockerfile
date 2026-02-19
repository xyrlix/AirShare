# 构建阶段
FROM node:18-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装Flutter依赖
RUN apk add --no-cache bash git unzip

# 安装Flutter
RUN git clone https://github.com/flutter/flutter.git /usr/local/flutter
ENV PATH="$PATH:/usr/local/flutter/bin"

# 验证Flutter安装
RUN flutter doctor

# 复制依赖文件
COPY pubspec.yaml ./
RUN flutter pub get

# 复制源代码
COPY . .

# 构建Web应用
RUN flutter build web --release

# 运行阶段
FROM nginx:alpine

# 复制构建结果到nginx目录
COPY --from=builder /app/build/web /usr/share/nginx/html

# 复制nginx配置
COPY nginx.conf /etc/nginx/nginx.conf

# 暴露端口
EXPOSE 80

# 启动nginx
CMD ["nginx", "-g", "daemon off;"]