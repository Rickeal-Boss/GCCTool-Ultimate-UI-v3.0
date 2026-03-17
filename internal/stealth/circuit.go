// Package stealth - 自适应熔断器与退避策略
//
// 熔断器状态机：
//   Closed（正常） → 失败累计到阈值 → Open（熔断）
//   Open（熔断）  → 冷却时间结束   → HalfOpen（半开）
//   HalfOpen      → 成功一次       → Closed
//   HalfOpen      → 失败一次       → Open（重置冷却计时）
//
// 设计原则：
//   - 熔断器是"最后一道防线"，防止在账号被限流期间持续轰炸系统
//   - 指数退避在熔断器之外作为"软控制"层
//   - 两者协同工作：指数退避 → 若持续失败 → 熔断器彻底切断
package stealth

import (
	"fmt"
	"sync"
	"time"
)

// CircuitState 熔断器状态
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // 正常工作
	CircuitOpen                         // 熔断中，拒绝所有请求
	CircuitHalfOpen                     // 试探恢复中
)

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	mu sync.Mutex

	state        CircuitState
	failureCount int
	successCount int // HalfOpen 阶段连续成功次数

	// 配置参数
	failureThreshold int           // 连续失败多少次后开路（默认 5）
	successThreshold int           // HalfOpen 阶段需要连续成功多少次才关路（默认 2）
	cooldownDuration time.Duration // 开路后冷却多久才进入 HalfOpen（默认 30s）
	maxCooldown      time.Duration // 最大冷却时间（防无限增长，默认 5min）

	openedAt        time.Time
	currentCooldown time.Duration // 当前实际冷却时间（指数增长）

	name string // 标识符，用于日志
}

// NewCircuitBreaker 创建熔断器（使用默认参数）
func NewCircuitBreaker(name string) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		failureThreshold: 5,
		successThreshold: 2,
		cooldownDuration: 30 * time.Second,
		maxCooldown:      5 * time.Minute,
		currentCooldown:  30 * time.Second,
	}
}

// NewCircuitBreakerWithConfig 创建可配置熔断器
func NewCircuitBreakerWithConfig(name string, failThreshold, successThreshold int, cooldown, maxCooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		failureThreshold: failThreshold,
		successThreshold: successThreshold,
		cooldownDuration: cooldown,
		maxCooldown:      maxCooldown,
		currentCooldown:  cooldown,
	}
}

// ErrCircuitOpen 熔断器开路错误
type ErrCircuitOpen struct {
	Name      string
	ResetAt   time.Time
	Remaining time.Duration
}

func (e *ErrCircuitOpen) Error() string {
	return fmt.Sprintf("熔断器[%s]已开路，请在 %s 后重试（剩余 %.0f 秒）",
		e.Name, e.ResetAt.Format("15:04:05"), e.Remaining.Seconds())
}

// Allow 检查是否允许本次请求通过
//
// 返回 nil 表示允许，返回 *ErrCircuitOpen 表示拒绝。
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return nil

	case CircuitOpen:
		resetAt := cb.openedAt.Add(cb.currentCooldown)
		remaining := time.Until(resetAt)
		if remaining > 0 {
			return &ErrCircuitOpen{
				Name:      cb.name,
				ResetAt:   resetAt,
				Remaining: remaining,
			}
		}
		// 冷却时间结束，进入半开状态
		cb.state = CircuitHalfOpen
		cb.successCount = 0
		return nil

	case CircuitHalfOpen:
		return nil
	}

	return nil
}

// RecordSuccess 记录一次成功
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			// 恢复正常
			cb.state = CircuitClosed
			cb.failureCount = 0
			cb.successCount = 0
			cb.currentCooldown = cb.cooldownDuration // 重置冷却时间
		}
	case CircuitClosed:
		cb.failureCount = 0 // 成功则清零失败计数
	}
}

// RecordFailure 记录一次失败
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		cb.failureCount++
		if cb.failureCount >= cb.failureThreshold {
			cb.state = CircuitOpen
			cb.openedAt = time.Now()
		}
	case CircuitHalfOpen:
		// 半开状态失败，重新开路，冷却时间指数增长
		cb.state = CircuitOpen
		cb.openedAt = time.Now()
		cb.currentCooldown = min2(cb.currentCooldown*2, cb.maxCooldown)
		cb.successCount = 0
	}
}

// State 返回当前状态（线程安全）
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Reset 强制重置熔断器（用户手动重置）
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.currentCooldown = cb.cooldownDuration
}

// StateName 返回状态名称
func (cb *CircuitBreaker) StateName() string {
	switch cb.State() {
	case CircuitClosed:
		return "正常"
	case CircuitOpen:
		return "熔断中"
	case CircuitHalfOpen:
		return "恢复中"
	default:
		return "未知"
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 指数退避策略（与熔断器协同工作）
// ─────────────────────────────────────────────────────────────────────────────

// BackoffStrategy 退避策略
type BackoffStrategy struct {
	mu sync.Mutex

	current  time.Duration
	base     time.Duration
	maxDelay time.Duration
	factor   float64 // 退避乘数，默认 2.0
	jitter   bool    // 是否添加随机抖动
}

// NewBackoffStrategy 创建退避策略
// base: 初始延迟; max: 最大延迟; factor: 乘数(建议2.0); jitter: 是否加抖动
func NewBackoffStrategy(base, max time.Duration, factor float64, jitter bool) *BackoffStrategy {
	return &BackoffStrategy{
		current:  base,
		base:     base,
		maxDelay: max,
		factor:   factor,
		jitter:   jitter,
	}
}

// Next 获取下次退避时间并递增
func (b *BackoffStrategy) Next() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	delay := b.current

	if b.jitter {
		// 添加 ±20% 随机抖动
		jitterRange := int64(float64(delay) * 0.2)
		if jitterRange > 0 {
			delta := time.Duration(rand.Int63n(jitterRange*2) - jitterRange)
			delay += delta
		}
	}

	// 递增
	next := time.Duration(float64(b.current) * b.factor)
	if next > b.maxDelay {
		next = b.maxDelay
	}
	b.current = next

	if delay < 0 {
		delay = b.base
	}
	return delay
}

// Reset 重置退避到初始值
func (b *BackoffStrategy) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.current = b.base
}

// Current 查看当前退避时间（不递增）
func (b *BackoffStrategy) Current() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.current
}

// ─────────────────────────────────────────────────────────────────────────────
// 辅助函数
// ─────────────────────────────────────────────────────────────────────────────

func min2(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
