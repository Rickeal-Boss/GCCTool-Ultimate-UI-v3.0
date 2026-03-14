//go:build prod

package logger

// isProdBuild 在生产构建中为 true，控制台日志输出将被关闭。
// 使用方式：go build -tags prod ./...
const isProdBuild = true
