package main

import (
	"fmt"
	"os"
	"path/filepath"
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

// errorLogPath 返回崩溃日志路径（与可执行文件同目录）
//
// 修复（V3.0 bug）：原实现写死相对路径 "error.log"，即落在"当前工作目录"——
// 从桌面快捷方式或其他目录启动时，日志会散落到任意位置，与
// gcctool_config.json 的"exe 同目录"策略不一致；同时完全忽略 WriteFile
// 的返回值，写入失败（目录只读等）会被静默吞掉，排查崩溃时反而找不到日志。
func errorLogPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "error.log"
	}
	return filepath.Join(filepath.Dir(exe), "error.log")
}

func main() {
	// 全局错误恢复
	defer func() {
		if r := recover(); r != nil {
			raw := fmt.Sprintf("程序异常:\n%v\n\n堆栈信息:\n%s", r, debug.Stack())

			// 过滤敏感字段后再输出/落盘
			safe := sanitizeStack(raw)
			fmt.Fprintln(os.Stderr, safe)

			// 权限 0600（仅当前用户可读；Windows 会忽略该权限位，此处保持跨平台一致）
			path := errorLogPath()
			if err := os.WriteFile(path, []byte(safe), 0600); err != nil {
				fmt.Fprintf(os.Stderr, "写入崩溃日志失败(%s): %v\n", path, err)
			} else {
				fmt.Fprintf(os.Stderr, "崩溃日志已写入: %s\n", path)
			}

			os.Exit(1)
		}
	}()

	// 创建应用
	app := ui.NewApp()

	// 运行
	app.Run()
}
