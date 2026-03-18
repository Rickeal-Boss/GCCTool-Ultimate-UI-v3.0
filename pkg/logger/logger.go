package logger

import (
	"fmt"
	"sync"
	"time"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
	"github.com/atotto/clipboard"
)

// LogLevel 日志级别
type LogLevel int

const (
	LevelInfo LogLevel = iota
	LevelWarn
	LevelError
	LevelSuccess
)

// Logger 日志器
type Logger struct {
	ui      *model.UIComponents
	logChan chan logMessage
	mu      sync.Mutex
	running bool
}

type logMessage struct {
	level   LogLevel
	message string
	time    time.Time
}

// NewLogger 创建日志器
func NewLogger(ui *model.UIComponents) *Logger {
	l := &Logger{
		ui:      ui,
		logChan: make(chan logMessage, 100),
	}

	l.running = true
	go l.processLogs()

	return l
}

// Close 关闭日志器
func (l *Logger) Close() {
	l.running = false
	close(l.logChan)
}

// processLogs 处理日志（单 goroutine 串行消费，避免并发写 UI）
func (l *Logger) processLogs() {
	for msg := range l.logChan {
		// UI 文本：无 ANSI 转义码（widget.Label 不渲染转义，会显示为乱码）
		uiText := l.formatLogUI(msg)
		l.ui.AppendLog(uiText)

		// 终端输出：带 ANSI 颜色（仅调试构建；生产构建通过 isProdBuild 关闭）
		// stdout 输出含学号/选课信息，生产构建必须关闭
		if !isProdBuild {
			fmt.Println(l.formatLogTerminal(msg))
		}
	}
}

// formatLogUI 格式化日志文本（供 widget.Label 显示，不含 ANSI 转义）
//
// 使用语义化前缀符号增强日志可读性：
//   ·  INFO  — 普通信息（低调小点）
//   ⚠  WARN  — 警告（醒目三角）
//   ✕  ERROR — 错误（叉号，易识别）
//   ✓  成功  — 成功（勾号，正向反馈）
func (l *Logger) formatLogUI(msg logMessage) string {
	timestamp := msg.time.Format("15:04:05")
	var prefix string
	switch msg.level {
	case LevelInfo:
		prefix = "·"
	case LevelWarn:
		prefix = "⚠"
	case LevelError:
		prefix = "✕"
	case LevelSuccess:
		prefix = "✓"
	}
	return fmt.Sprintf("%s %s %s", timestamp, prefix, msg.message)
}

// formatLogTerminal 格式化日志文本（供终端输出，含 ANSI 颜色）
// 仅在 isProdBuild=false 时使用
func (l *Logger) formatLogTerminal(msg logMessage) string {
	timestamp := msg.time.Format("15:04:05")
	var level, ansiColor, reset string
	reset = "\033[0m"
	switch msg.level {
	case LevelInfo:
		level = "[INFO]"
		ansiColor = "\033[0m"
	case LevelWarn:
		level = "[WARN]"
		ansiColor = "\033[33m"
	case LevelError:
		level = "[ERROR]"
		ansiColor = "\033[31m"
	case LevelSuccess:
		level = "[成功]"
		ansiColor = "\033[32m"
	}
	return fmt.Sprintf("%s %s%s%s %s", timestamp, ansiColor, level, reset, msg.message)
}

// Info 信息日志
func (l *Logger) Info(message string) {
	l.log(LevelInfo, message)
}

// Warn 警告日志
func (l *Logger) Warn(message string) {
	l.log(LevelWarn, message)
}

// Error 错误日志
func (l *Logger) Error(message string) {
	l.log(LevelError, message)
}

// Success 成功日志
func (l *Logger) Success(message string) {
	l.log(LevelSuccess, message)
}

// log 记录日志
func (l *Logger) log(level LogLevel, message string) {
	if !l.running {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	select {
	case l.logChan <- logMessage{
		level:   level,
		message: message,
		time:    time.Now(),
	}:
	default:
		// 队列满，丢弃旧日志
	}
}

// Copy 复制日志到剪贴板
func (l *Logger) Copy() bool {
	if l.ui == nil {
		return false
	}

	text := l.ui.LogLabel.Text
	if text == "" {
		return false
	}

	err := clipboard.WriteAll(text)
	return err == nil
}

// Clear 清空日志
func (l *Logger) Clear() {
	if l.ui != nil {
		l.ui.ClearLog()
	}
}
