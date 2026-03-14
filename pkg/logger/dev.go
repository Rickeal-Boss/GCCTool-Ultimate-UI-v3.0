//go:build !prod

package logger

// isProdBuild 在调试构建中为 false，控制台日志输出保持开启，方便开发调试。
// 生产构建：go build -tags prod ./...
// 调试构建：go build ./...（默认）
const isProdBuild = false
