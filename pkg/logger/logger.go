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

	// clearSentinel 用于通过 logChan 发送"清空日志"指令，
	// 保证 Clear() 也在 processLogs goroutine 内串行执行，不与 AppendLog 竞争。
	clearSentinel LogLevel = -1

	// copySentinel 用于通过 logChan 发送"复制日志"指令，
	// 保证 Copy() 在 processLogs goroutine 内串行读取 logLines，不与 AppendLog 竞争。
	copySentinel LogLevel = -2
)

// Logger 日志器
type Logger struct {
	ui      *model.UIComponents
	logChan chan logMessage
	mu      sync.Mutex // 保护 running 标志位的写入，以及 channel 发送的原子性
	running bool
	once    sync.Once // 保证 Close() 只关闭一次 channel
}

type logMessage struct {
	level    LogLevel
	message  string
	time     time.Time
	resultCh chan string // 仅 copySentinel 使用，用于回传日志文本
}

// NewLogger 创建日志器
func NewLogger(ui *model.UIComponents) *Logger {
	l := &Logger{
		ui:      ui,
		logChan: make(chan logMessage, 100),
	}

	l.mu.Lock()
	l.running = true
	l.mu.Unlock()

	go l.processLogs()

	return l
}

// Close 关闭日志器，停止后台 goroutine。
//
// 线程安全：可以被多次调用，但只有第一次调用真正关闭 channel。
// 关闭后任何对 Info/Warn/Error/Success/Clear 的调用都会静默丢弃。
func (l *Logger) Close() {
	l.once.Do(func() {
		l.mu.Lock()
		l.running = false
		l.mu.Unlock()
		close(l.logChan)
	})
}

// processLogs 处理日志（单 goroutine 串行消费，避免并发写 UI）
func (l *Logger) processLogs() {
	for msg := range l.logChan {
		switch msg.level {
		case clearSentinel:
			// 清空日志指令
			l.ui.ClearLog()
		case copySentinel:
			// 复制日志指令：在此 goroutine 内串行读取，回传给 Copy() 调用方
			text := l.ui.LogLabel.Text
			if msg.resultCh != nil {
				msg.resultCh <- text
			}
		default:
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
}

// formatLogUI 格式化日志文本（供 widget.Label 显示，不含 ANSI 转义）
func (l *Logger) formatLogUI(msg logMessage) string {
	timestamp := msg.time.Format("15:04:05")
	var level string
	switch msg.level {
	case LevelInfo:
		level = "[INFO]"
	case LevelWarn:
		level = "[WARN]"
	case LevelError:
		level = "[ERROR]"
	case LevelSuccess:
		level = "[成功]"
	}
	return fmt.Sprintf("%s %s %s", timestamp, level, msg.message)
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

// log 记录日志（线程安全）
//
// 持锁后再检查 running，保证 Close() 设置 running=false 并 close(channel) 的原子性，
// 避免在 channel 已关闭后仍然向其发送消息（panic: send on closed channel）。
func (l *Logger) log(level LogLevel, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.running {
		return
	}

	select {
	case l.logChan <- logMessage{
		level:   level,
		message: message,
		time:    time.Now(),
	}:
	default:
		// 队列满，丢弃本条日志
	}
}

// Copy 复制日志到剪贴板
//
// 安全说明：
//   - 通过 logChan 发送 copyRequest 到 processLogs goroutine 串行执行，
//     避免与 AppendLog/ClearLog 并发读写 logLines 切片。
//   - 日志中含有学号、选课行为等敏感信息，复制前 UI 层（ui/main.go）已弹出确认对话框。
func (l *Logger) Copy() bool {
	if l.ui == nil {
		return false
	}

	// 通过带结果回传的 channel 在 processLogs goroutine 里串行读取文本
	resultCh := make(chan string, 1)

	l.mu.Lock()
	if !l.running {
		l.mu.Unlock()
		return false
	}
	select {
	case l.logChan <- logMessage{level: copySentinel, resultCh: resultCh}:
	default:
		l.mu.Unlock()
		return false
	}
	l.mu.Unlock()

	text := <-resultCh
	if text == "" {
		return false
	}
	return clipboard.WriteAll(text) == nil
}

// Clear 清空日志
//
// 通过 logChan 发送清空指令，由 processLogs goroutine 串行执行，
// 避免与 AppendLog 并发操作 logLines 切片。
func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.running {
		return
	}

	select {
	case l.logChan <- logMessage{level: clearSentinel}:
	default:
		// 队列满时丢弃清空指令（不会导致崩溃，只是日志没被清空）
	}
}
