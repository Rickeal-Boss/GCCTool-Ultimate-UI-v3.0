# GCC选课助手 V3.0 - Windows构建脚本

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "GCC选课助手 V3.0 - Windows构建脚本" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 设置版本号
$VERSION = if ($env:VERSION) { $env:VERSION } else { "3.0.0" }
$BUILD_TIME = Get-Date -Format "yyyyMMdd-HHmmss"

# 设置Go环境
$env:CGO_ENABLED = "1"

Write-Host "[1/4] 检查Go版本..." -ForegroundColor Yellow
go version

Write-Host ""
Write-Host "[2/4] 下载依赖..." -ForegroundColor Yellow
go mod download

Write-Host ""
Write-Host "[3/4] 编译程序..." -ForegroundColor Yellow
$LDFLAGS = "-s -w -X main.version=$VERSION -X main.buildTime=$BUILD_TIME"

# -tags prod：关闭控制台日志输出，避免学号/选课信息泄露到标准输出
go build -v -tags prod -ldflags=$LDFLAGS -o "gcc_helper_v${VERSION}.exe" main.go

Write-Host ""
Write-Host "[4/4] 构建完成！" -ForegroundColor Green

# 显示文件信息
if (Test-Path "gcc_helper_v${VERSION}.exe") {
    $SIZE = [math]::Round((Get-Item "gcc_helper_v${VERSION}.exe").Length / 1MB, 2)
    Write-Host "----------------------------------------" -ForegroundColor Cyan
    Write-Host "文件名: gcc_helper_v${VERSION}.exe"
    Write-Host "文件大小: $SIZE MB"
    Write-Host "版本: $VERSION"
    Write-Host "构建时间: $BUILD_TIME"
    Write-Host "----------------------------------------" -ForegroundColor Cyan
} else {
    Write-Host "错误: 构建失败！" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "✓ 构建成功！" -ForegroundColor Green
