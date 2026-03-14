#!/bin/bash

# GCC选课助手 V3.0 - 构建脚本

set -e

echo "========================================"
echo "GCC选课助手 V3.0 - 构建脚本"
echo "========================================"
echo ""

# 设置版本号
VERSION=${VERSION:-"3.0.0"}
BUILD_TIME=$(date +%Y%m%d-%H%M%S)

# 设置Go环境
export CGO_ENABLED=1

echo "[1/4] 检查Go版本..."
go version

echo ""
echo "[2/4] 下载依赖..."
go mod download

echo ""
echo "[3/4] 编译程序..."
LDFLAGS="-s -w -X main.version=$VERSION -X main.buildTime=$BUILD_TIME"

# -tags prod：关闭控制台日志输出，避免学号/选课信息泄露到标准输出
go build -v -tags prod -ldflags="$LDFLAGS" -o "gcc_helper_v${VERSION}.exe" main.go

echo ""
echo "[4/4] 构建完成！"

# 显示文件信息
if [ -f "gcc_helper_v${VERSION}.exe" ]; then
    SIZE=$(du -h "gcc_helper_v${VERSION}.exe" | cut -f1)
    echo "----------------------------------------"
    echo "文件名: gcc_helper_v${VERSION}.exe"
    echo "文件大小: $SIZE"
    echo "版本: $VERSION"
    echo "构建时间: $BUILD_TIME"
    echo "----------------------------------------"
else
    echo "错误: 构建失败！"
    exit 1
fi

echo ""
echo "✓ 构建成功！"
