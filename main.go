package main

import (
	"fmt"
	"os"
	"regexp"
	"runtime/debug"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/ui"
)

// sensitivePattern 匹配堆栈信息中可能残留的敏感字段值
// 仅做最保守的过滤：password= / passwd= / pwd= 后跟任意非空白内容
var sensitivePattern = regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[=:]\s*\S+`)

// sanitizeStack 过滤崩溃信息中的敏感字段，避免密码写入 error.log
func sanitizeStack(s string) string {
	return sensitivePattern.ReplaceAllString(s, "$1=***REDACTED***")
}

func main() {
	// 全局错误恢复
	defer func() {
		if r := recover(); r != nil {
			raw := fmt.Sprintf("程序异常:\n%v\n\n堆栈信息:\n%s", r, debug.Stack())

			// 过滤敏感字段后再输出/落盘
			safe := sanitizeStack(raw)
			fmt.Fprintln(os.Stderr, safe)

			// 权限改为 0600（仅当前用户可读），防止共享机器上其他用户读取
			os.WriteFile("error.log", []byte(safe), 0600)

			os.Exit(1)
		}
	}()

	// 创建应用
	app := ui.NewApp()

	// 运行
	app.Run()
}
