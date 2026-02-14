#!/bin/bash
# Docker 构建脚本 - 在任何环境中构建 Tauri 应用

set -e

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$PROJECT_DIR/tauri-app/src-tauri/nsis/skin-installer/Output"

echo "=========================================="
echo "Docker 构建 - ZimaOS Blue"
echo "=========================================="
echo ""

# 检查 Docker
if ! command -v docker &> /dev/null; then
    echo "❌ 错误: Docker 未安装"
    echo "请从 https://www.docker.com/products/docker-desktop 下载安装"
    exit 1
fi

echo "✓ Docker 已安装"
echo ""

# 创建输出目录
mkdir -p "$OUTPUT_DIR"

# 构建 Docker 镜像
echo "构建 Docker 镜像..."
docker build -t zimaos-blue-builder "$PROJECT_DIR/tauri-app" -f - << 'EOF'
FROM rust:1.75-slim-bookworm

RUN apt-get update && apt-get install -y \
    curl build-essential libssl-dev pkg-config \
    libgtk-3-dev libwebkit2gtk-4.1-dev libappindicator3-dev \
    librsvg2-dev patchelf && \
    rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y nodejs && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY . .
RUN npm install
RUN npm run tauri build -- --target x86_64-pc-windows-msvc
EOF

echo "✓ Docker 镜像构建完成"
echo ""

# 运行构建
echo "运行构建容器..."
docker run --rm \
    -v "$PROJECT_DIR/tauri-app:/app" \
    -v "$OUTPUT_DIR:/output" \
    zimaos-blue-builder \
    bash -c "cp -r src-tauri/nsis/skin-installer/Output/* /output/ 2>/dev/null || true"

echo "✓ 构建完成"
echo ""
echo "输出文件位置: $OUTPUT_DIR"
echo ""

# 列出输出文件
if [ -d "$OUTPUT_DIR" ] && [ "$(ls -A $OUTPUT_DIR)" ]; then
    echo "生成的文件:"
    ls -lh "$OUTPUT_DIR"
else
    echo "⚠️  未找到输出文件"
fi
