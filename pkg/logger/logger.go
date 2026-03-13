package logger

import (
	"fmt"
	"sync"
	"time"

	"gcctool/internal/model"
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

// processLogs 处理日志
func (l *Logger) processLogs() {
	for msg := range l.logChan {
		// 格式化日志
		formatted := l.formatLog(msg)

		// 输出到UI
		l.ui.AppendLog(formatted)

		// 输出到控制台
		fmt.Println(formatted)
	}
}

// formatLog 格式化日志
func (l *Logger) formatLog(msg logMessage) string {
	timestamp := msg.time.Format("15:04:05")

	var level string
	var color string
	switch msg.level {
	case LevelInfo:
		level = "[INFO]"
		color = "\033[0m"
	case LevelWarn:
		level = "[WARN]"
		color = "\033[33m"
	case LevelError:
		level = "[ERROR]"
		color = "\033[31m"
	case LevelSuccess:
		level = "[SUCCESS]"
		color = "\033[32m"
	}

	return fmt.Sprintf("%s %s %s %s", timestamp, color, level, msg.message)
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
