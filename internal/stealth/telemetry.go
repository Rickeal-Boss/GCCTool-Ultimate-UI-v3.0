// Package stealth - 遥测日志与风控统计
//
// 作用：为自主优化架构提供运行时遥测数据，
// 记录每次请求的成本（时间）、风险信号，供后续分析路由策略优化。
package stealth

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// 请求遥测记录
// ─────────────────────────────────────────────────────────────────────────────

// RequestRecord 单次请求遥测记录
type RequestRecord struct {
	Timestamp  time.Time
	URL        string
	Method     string
	StatusCode int
	Latency    time.Duration
	RiskLevel  RiskLevel
	Error      string
	UA         string // 本次使用的 User-Agent（方便分析哪个 UA 更容易触发风控）
}

// ─────────────────────────────────────────────────────────────────────────────
// 全局遥测收集器
// ─────────────────────────────────────────────────────────────────────────────

// Telemetry 遥测收集器（线程安全）
type Telemetry struct {
	mu      sync.Mutex
	records []RequestRecord

	// 原子计数器（高频读写用 atomic 避免锁竞争）
	totalRequests   int64
	totalErrors     int64
	rateLimitHits   int64
	sessionExpiries int64
	bannedSignals   int64
	captchaSignals  int64
	successSelects  int64
}

// Global 全局遥测实例（单例）
var Global = &Telemetry{}

// Record 记录一次请求遥测
func (t *Telemetry) Record(r RequestRecord) {
	atomic.AddInt64(&t.totalRequests, 1)

	if r.Error != "" {
		atomic.AddInt64(&t.totalErrors, 1)
	}

	switch r.RiskLevel {
	case RiskRateLimit:
		atomic.AddInt64(&t.rateLimitHits, 1)
	case RiskSessionExpired:
		atomic.AddInt64(&t.sessionExpiries, 1)
	case RiskBanned:
		atomic.AddInt64(&t.bannedSignals, 1)
	case RiskCaptcha:
		atomic.AddInt64(&t.captchaSignals, 1)
	case RiskSelectSuccess:
		atomic.AddInt64(&t.successSelects, 1)
	}

	// 只保留最近 200 条详细记录（内存控制）
	t.mu.Lock()
	t.records = append(t.records, r)
	if len(t.records) > 200 {
		t.records = t.records[len(t.records)-200:]
	}
	t.mu.Unlock()
}

// Summary 返回遥测摘要（可展示在 UI 日志中）
func (t *Telemetry) Summary() string {
	total := atomic.LoadInt64(&t.totalRequests)
	errors := atomic.LoadInt64(&t.totalErrors)
	limits := atomic.LoadInt64(&t.rateLimitHits)
	sessions := atomic.LoadInt64(&t.sessionExpiries)
	banned := atomic.LoadInt64(&t.bannedSignals)
	captcha := atomic.LoadInt64(&t.captchaSignals)
	success := atomic.LoadInt64(&t.successSelects)

	if total == 0 {
		return "暂无遥测数据"
	}

	errorRate := float64(errors) / float64(total) * 100
	return fmt.Sprintf(
		"📊 遥测摘要 | 总请求: %d | 错误率: %.1f%% | 限流次数: %d | Session过期: %d | 封号信号: %d | 验证码: %d | 选课成功: %d",
		total, errorRate, limits, sessions, banned, captcha, success,
	)
}

// Reset 重置遥测数据（新任务开始时调用）
func (t *Telemetry) Reset() {
	atomic.StoreInt64(&t.totalRequests, 0)
	atomic.StoreInt64(&t.totalErrors, 0)
	atomic.StoreInt64(&t.rateLimitHits, 0)
	atomic.StoreInt64(&t.sessionExpiries, 0)
	atomic.StoreInt64(&t.bannedSignals, 0)
	atomic.StoreInt64(&t.captchaSignals, 0)
	atomic.StoreInt64(&t.successSelects, 0)
	t.mu.Lock()
	t.records = t.records[:0]
	t.mu.Unlock()
}

// ─────────────────────────────────────────────────────────────────────────────
// 自主优化顾问
//
// 基于历史遥测数据，自动给出路由/策略优化建议
// ─────────────────────────────────────────────────────────────────────────────

// StrategyAdvice 策略建议
type StrategyAdvice struct {
	Severity    string // "INFO" / "WARN" / "CRITICAL"
	Description string
	Action      string
}

// Analyze 分析遥测数据，给出策略建议
//
// 触发条件（类比 FinOps 护栏）：
//   - 限流率 > 30% → 建议降低线程数/增大延迟
//   - Session 过期率 > 10% → 建议缩短保活间隔
//   - 封号信号 > 0 → 强烈警告，建议立即停止
//   - 验证码信号 > 0 → 建议切换节点
func (t *Telemetry) Analyze() []StrategyAdvice {
	var advices []StrategyAdvice

	total := atomic.LoadInt64(&t.totalRequests)
	if total < 10 {
		return nil // 样本量太少，不分析
	}

	limits := atomic.LoadInt64(&t.rateLimitHits)
	sessions := atomic.LoadInt64(&t.sessionExpiries)
	banned := atomic.LoadInt64(&t.bannedSignals)
	captcha := atomic.LoadInt64(&t.captchaSignals)

	// 封号信号（最高优先级）
	if banned > 0 {
		advices = append(advices, StrategyAdvice{
			Severity:    "CRITICAL",
			Description: fmt.Sprintf("检测到 %d 次封号信号", banned),
			Action:      "立即停止所有任务，手动登录教务系统检查账号状态",
		})
	}

	// 验证码信号
	if captcha > 0 {
		advices = append(advices, StrategyAdvice{
			Severity:    "CRITICAL",
			Description: fmt.Sprintf("检测到 %d 次验证码触发", captcha),
			Action:      "切换到不同节点，增大请求间隔，考虑减少并发线程数",
		})
	}

	// 限流率 > 30%
	limitRate := float64(limits) / float64(total)
	if limitRate > 0.30 {
		advices = append(advices, StrategyAdvice{
			Severity:    "WARN",
			Description: fmt.Sprintf("限流触发率 %.1f%%（阈值 30%%）", limitRate*100),
			Action:      "建议将线程数降低 50%，增大基础延迟至 500ms+",
		})
	}

	// Session 过期率 > 10%
	sessionRate := float64(sessions) / float64(total)
	if sessionRate > 0.10 {
		advices = append(advices, StrategyAdvice{
			Severity:    "WARN",
			Description: fmt.Sprintf("Session 过期率 %.1f%%（阈值 10%%）", sessionRate*100),
			Action:      "缩短 Session 保活间隔至 2 分钟",
		})
	}

	return advices
}

// FormatAdvices 将策略建议格式化为可显示的字符串
func FormatAdvices(advices []StrategyAdvice) string {
	if len(advices) == 0 {
		return "✅ 当前运行状态良好，无策略调整建议"
	}
	result := "⚙️ 自主优化建议：\n"
	for _, a := range advices {
		result += fmt.Sprintf("[%s] %s → %s\n", a.Severity, a.Description, a.Action)
	}
	return result
}
