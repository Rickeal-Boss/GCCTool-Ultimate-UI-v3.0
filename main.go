package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/ui"
)

func main() {
	// 全局错误恢复
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("程序异常:\n%v\n\n堆栈信息:\n%s", r, debug.Stack())
			fmt.Fprintln(os.Stderr, errMsg)

			// 尝试保存到文件
			os.WriteFile("error.log", []byte(errMsg), 0644)

			os.Exit(1)
		}
	}()

	// 创建应用
	app := ui.NewApp()

	// 运行
	app.Run()
}
