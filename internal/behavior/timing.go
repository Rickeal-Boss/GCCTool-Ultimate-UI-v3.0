// Package behavior - 行为模拟模块
//
// 模拟真实用户的操作行为，防止被教务系统识别为自动化工具
package behavior

import (
	"math/rand"
	"time"
)

// TimingManager 时间管理器
//
// 管理操作之间的时间间隔，模拟人类的操作节奏
type TimingManager struct {
	baseDelay time.Duration
	variance  time.Duration
	lastAction time.Time
}

// NewTimingManager 创建时间管理器
//
// 参数：
//   - baseDelay: 基础延迟时间
//   - variance: 延迟方差（随机抖动范围）
//
// 返回：时间管理器实例
func NewTimingManager(baseDelay, variance time.Duration) *TimingManager {
	return &TimingManager{
		baseDelay: baseDelay,
		variance:  variance,
	}
}

// Wait 等待下一个操作
//
// 计算并等待下一个操作的延迟时间
// 延迟时间 = baseDelay + random(-variance, +variance)
func (t *TimingManager) Wait() {
	delay := t.CalculateDelay()
	time.Sleep(delay)
	t.lastAction = time.Now()
}

// CalculateDelay 计算延迟时间
//
// 计算下一个操作的延迟时间
// 延迟时间 = baseDelay + random(-variance, +variance)
func (t *TimingManager) CalculateDelay() time.Duration {
	variance := int64(t.variance)
	randomOffset := rand.Int63n(2*variance) - variance
	return t.baseDelay + time.Duration(randomOffset)
}

// Reset 重置时间管理器
//
// 重置最后操作时间
func (t *TimingManager) Reset() {
	t.lastAction = time.Now()
}

// TimeSinceLastAction 计算距离上次操作的时间
//
// 返回距离上次操作的时间间隔
func (t *TimingManager) TimeSinceLastAction() time.Duration {
	return time.Since(t.lastAction)
}

// HumanSessionInterval 模拟人类会话间隔
//
// 模拟人类在不同会话之间的时间间隔
// 返回：10~60 秒
func HumanSessionInterval() time.Duration {
	return time.Duration(10+rand.Intn(50)) * time.Second
}

// HumanBreakInterval 模拟人类休息间隔
//
// 模拟人类在工作过程中的休息时间
// 返回：1~5 分钟
func HumanBreakInterval() time.Duration {
	return time.Duration(60+rand.Intn(240)) * time.Second
}

// HumanWorkDuration 模拟人类工作持续时间
//
// 模拟人类连续工作的持续时间
// 返回：5~15 分钟
func HumanWorkDuration() time.Duration {
	return time.Duration(300+rand.Intn(600)) * time.Second
}

// HumanActivityPattern 模拟人类活动模式
//
// 返回一个时间函数，模拟人类的活动模式
// 例如：早上 9:00 - 11:00 活跃，下午 2:00 - 5:00 活跃
func HumanActivityPattern() func(time.Time) bool {
	return func(t time.Time) bool {
		hour := t.Hour()

		// 早上 9:00 - 11:00 活跃
		if hour >= 9 && hour < 11 {
			return true
		}

		// 下午 2:00 - 5:00 活跃
		if hour >= 14 && hour < 17 {
			return true
		}

		// 晚上 7:00 - 10:00 活跃
		if hour >= 19 && hour < 22 {
			return true
		}

		return false
	}
}

// HumanPeaksHours 模拟人类高峰时段
//
// 返回一个函数，判断当前时间是否是高峰时段
// 高峰时段：选课开始前 5 分钟到选课开始后 15 分钟
func HumanPeaksHours(startTime time.Time) func(time.Time) bool {
	return func(t time.Time) bool {
		duration := t.Sub(startTime)
		// 选课开始前 5 分钟到选课开始后 15 分钟
		return duration >= -5*time.Minute && duration <= 15*time.Minute
	}
}

// AdaptiveTiming 自适应时间管理
//
// 根据教务系统的响应动态调整操作频率
type AdaptiveTiming struct {
	baseRate       float64 // 基础请求频率（请求/秒）
	currentRate    float64 // 当前请求频率
	riskLevel      float64 // 风险等级 (0~1)
	responseTime   time.Duration // 平均响应时间
	successRate    float64 // 成功率 (0~1)
}

// NewAdaptiveTiming 创建自适应时间管理器
//
// 参数：
//   - baseRate: 基础请求频率（请求/秒）
//
// 返回：自适应时间管理器实例
func NewAdaptiveTiming(baseRate float64) *AdaptiveTiming {
	return &AdaptiveTiming{
		baseRate:    baseRate,
		currentRate: baseRate,
		riskLevel:   0.0,
	}
}

// CalculateInterval 计算操作间隔
//
// 根据当前请求频率计算操作间隔
// 返回：操作间隔（毫秒）
func (a *AdaptiveTiming) CalculateInterval() time.Duration {
	intervalMs := 1000.0 / a.currentRate
	return time.Duration(intervalMs) * time.Millisecond
}

// UpdateRiskLevel 更新风险等级
//
// 根据风控信号更新风险等级
// 参数：
//   - riskLevel: 风险等级 (0~1)
func (a *AdaptiveTiming) UpdateRiskLevel(riskLevel float64) {
	a.riskLevel = riskLevel

	// 根据风险等级动态调整频率
	if riskLevel > 0.8 {
		// 高风险：大幅降速
		a.currentRate = a.baseRate * 0.1
	} else if riskLevel > 0.5 {
		// 中等风险：适度降速
		a.currentRate = a.baseRate * 0.5
	} else if riskLevel > 0.2 {
		// 低风险：轻微降速
		a.currentRate = a.baseRate * 0.8
	} else {
		// 无风险：逐步恢复到基础频率
		if a.currentRate < a.baseRate {
			a.currentRate *= 1.1 // 每次增加 10%
		}
		if a.currentRate > a.baseRate {
			a.currentRate = a.baseRate
		}
	}
}

// UpdateResponseTime 更新响应时间
//
// 根据教务系统的响应时间调整频率
// 参数：
//   - responseTime: 响应时间
func (a *AdaptiveTiming) UpdateResponseTime(responseTime time.Duration) {
	a.responseTime = responseTime

	// 如果响应时间过长，降速
	if responseTime > 2*time.Second {
		a.currentRate *= 0.8
	}
	// 如果响应时间很短，可以提速
	if responseTime < 500*time.Millisecond {
		if a.currentRate < a.baseRate*1.5 {
			a.currentRate *= 1.1
		}
	}
}

// UpdateSuccessRate 更新成功率
//
// 根据成功率调整频率
// 参数：
//   - successRate: 成功率 (0~1)
func (a *AdaptiveTiming) UpdateSuccessRate(successRate float64) {
	a.successRate = successRate

	// 如果成功率太低，降速
	if successRate < 0.5 {
		a.currentRate *= 0.7
	}
	// 如果成功率很高，可以提速
	if successRate > 0.9 {
		if a.currentRate < a.baseRate*1.2 {
			a.currentRate *= 1.05
		}
	}
}

// GetCurrentRate 获取当前请求频率
//
// 返回：当前请求频率（请求/秒）
func (a *AdaptiveTiming) GetCurrentRate() float64 {
	return a.currentRate
}
