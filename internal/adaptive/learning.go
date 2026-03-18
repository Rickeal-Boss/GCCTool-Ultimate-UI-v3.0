// Package adaptive - 智能适应模块
//
// 根据教务系统的响应动态调整策略，防止风控
package adaptive

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

// RiskSignalRecord 风控信号记录
type RiskSignalRecord struct {
	Timestamp      time.Time
	URL            string
	StatusCode     int
	Response       string
	TriggerKeyword string
	Context        map[string]interface{}
}

// RiskPattern 风险模式
type RiskPattern struct {
	Pattern   string
	TriggerCount int
	LastTriggered time.Time
}

// RiskSignalLearning 风控信号学习器
//
// 记录和分析风控信号，自动调整策略
type RiskSignalLearning struct {
	mu      sync.RWMutex
	signals []RiskSignalRecord
	patterns []RiskPattern
	maxSignals int
}

// NewRiskSignalLearning 创建风控信号学习器
//
// 参数：
//   - maxSignals: 最大记录数量
//
// 返回：风控信号学习器实例
func NewRiskSignalLearning(maxSignals int) *RiskSignalLearning {
	return &RiskSignalLearning{
		signals:    make([]RiskSignalRecord, 0, maxSignals),
		patterns:   make([]RiskPattern, 0),
		maxSignals: maxSignals,
	}
}

// RecordSignal 记录风控信号
//
// 参数：
//   - signal: 风控信号
func (r *RiskSignalLearning) RecordSignal(signal RiskSignalRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 添加记录
	r.signals = append(r.signals, signal)

	// 限制记录数量
	if len(r.signals) > r.maxSignals {
		r.signals = r.signals[1:]
	}

	// 分析模式
	r.analyzePattern(signal)
}

// analyzePattern 分析风控模式
//
// 从风控信号中提取模式
// 参数：
//   - signal: 风控信号
func (r *RiskSignalLearning) analyzePattern(signal RiskSignalRecord) {
	// 提取触发关键词作为模式
	pattern := signal.TriggerKeyword

	// 查找是否已存在该模式
	found := false
	for i, p := range r.patterns {
		if p.Pattern == pattern {
			r.patterns[i].TriggerCount++
			r.patterns[i].LastTriggered = signal.Timestamp
			found = true
			break
		}
	}

	// 如果不存在，添加新模式
	if !found {
		r.patterns = append(r.patterns, RiskPattern{
			Pattern:        pattern,
			TriggerCount:   1,
			LastTriggered:  signal.Timestamp,
		})
	}
}

// GetHighRiskPatterns 获取高风险模式
//
// 返回触发次数最多的前 5 个模式
// 返回：风险模式列表
func (r *RiskSignalLearning) GetHighRiskPatterns() []RiskPattern {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 按触发次数排序
	sorted := make([]RiskPattern, len(r.patterns))
	copy(sorted, r.patterns)

	// 简单排序（触发次数降序）
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].TriggerCount > sorted[i].TriggerCount {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// 返回前 5 个
	if len(sorted) > 5 {
		sorted = sorted[:5]
	}

	return sorted
}

// GetRecentSignals 获取最近的风控信号
//
// 参数：
//   - duration: 时间范围
//
// 返回：最近的风控信号列表
func (r *RiskSignalLearning) GetRecentSignals(duration time.Duration) []RiskSignalRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	recent := make([]RiskSignalRecord, 0)

	for _, signal := range r.signals {
		if signal.Timestamp.After(cutoff) {
			recent = append(recent, signal)
		}
	}

	return recent
}

// GetRiskFrequency 获取风险频率
//
// 计算指定时间范围内的风险频率
// 参数：
//   - duration: 时间范围
//
// 返回：风险频率（次/秒）
func (r *RiskSignalLearning) GetRiskFrequency(duration time.Duration) float64 {
	recent := r.GetRecentSignals(duration)
	return float64(len(recent)) / duration.Seconds()
}

// GetRiskTrend 获取风险趋势
//
// 计算风险的变化趋势
// 参数：
//   - duration: 时间范围
//
// 返回：风险趋势（1 = 增加，-1 = 减少，0 = 无变化）
func (r *RiskSignalLearning) GetRiskTrend(duration time.Duration) int {
	// 将时间范围分为两半
	halfDuration := duration / 2
	firstHalf := r.GetRecentSignals(duration - halfDuration)
	secondHalf := r.GetRecentSignals(halfDuration)

	firstCount := len(firstHalf)
	secondCount := len(secondHalf)

	// 比较两半的频率
	if secondCount > firstCount*1.2 {
		return 1 // 风险增加
	} else if secondCount < firstCount*0.8 {
		return -1 // 风险减少
	}
	return 0 // 风险稳定
}

// Clear 清除所有记录
func (r *RiskSignalLearning) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.signals = make([]RiskSignalRecord, 0, r.maxSignals)
	r.patterns = make([]RiskPattern, 0)
}

// AdaptiveStrategy 自适应策略
//
// 根据风控信号自动调整策略
type AdaptiveStrategy struct {
	riskLearning   *RiskSignalLearning
	baseDelay      time.Duration
	currentDelay   time.Duration
	minDelay       time.Duration
	maxDelay       time.Duration
	adjustFactor   float64
}

// NewAdaptiveStrategy 创建自适应策略
//
// 参数：
//   - baseDelay: 基础延迟
//   - minDelay: 最小延迟
//   - maxDelay: 最大延迟
//   - adjustFactor: 调整因子
//
// 返回：自适应策略实例
func NewAdaptiveStrategy(baseDelay, minDelay, maxDelay time.Duration, adjustFactor float64) *AdaptiveStrategy {
	return &AdaptiveStrategy{
		riskLearning: NewRiskSignalLearning(1000),
		baseDelay:    baseDelay,
		currentDelay: baseDelay,
		minDelay:     minDelay,
		maxDelay:     maxDelay,
		adjustFactor: adjustFactor,
	}
}

// GetDelay 获取当前延迟
//
// 返回：当前延迟时间
func (a *AdaptiveStrategy) GetDelay() time.Duration {
	return a.currentDelay
}

// OnRiskSignal 风控信号触发时的处理
//
// 参数：
//   - signal: 风控信号
func (a *AdaptiveStrategy) OnRiskSignal(signal RiskSignalRecord) {
	// 记录信号
	a.riskLearning.RecordSignal(signal)

	// 计算风险趋势
	trend := a.riskLearning.GetRiskTrend(5 * time.Minute)

	// 根据趋势调整延迟
	if trend == 1 {
		// 风险增加：增加延迟
		newDelay := time.Duration(float64(a.currentDelay) * (1 + a.adjustFactor))
		if newDelay > a.maxDelay {
			newDelay = a.maxDelay
		}
		a.currentDelay = newDelay
	} else if trend == -1 {
		// 风险减少：减少延迟
		newDelay := time.Duration(float64(a.currentDelay) * (1 - a.adjustFactor))
		if newDelay < a.minDelay {
			newDelay = a.minDelay
		}
		a.currentDelay = newDelay
	}
}

// OnSuccess 操作成功时的处理
//
// 参数：
//   - responseTime: 响应时间
func (a *AdaptiveStrategy) OnSuccess(responseTime time.Duration) {
	// 根据响应时间调整延迟
	if responseTime < 500*time.Millisecond {
		// 响应很快，可以提速
		newDelay := time.Duration(float64(a.currentDelay) * (1 - a.adjustFactor*0.5))
		if newDelay < a.minDelay {
			newDelay = a.minDelay
		}
		a.currentDelay = newDelay
	}
}

// GetRiskLevel 获取风险等级
//
// 计算当前风险等级
// 返回：风险等级 (0~1)
func (a *AdaptiveStrategy) GetRiskLevel() float64 {
	// 计算最近 1 分钟的风险频率
	frequency := a.riskLearning.GetRiskFrequency(time.Minute)

	// 频率转换为风险等级
	riskLevel := frequency * 10.0 // 假设 0.1 次/秒对应风险等级 1

	if riskLevel > 1 {
		riskLevel = 1
	}

	return riskLevel
}

// ShouldIncreaseDelay 判断是否应该增加延迟
//
// 返回：是否应该增加延迟
func (a *AdaptiveStrategy) ShouldIncreaseDelay() bool {
	riskLevel := a.GetRiskLevel()
	trend := a.riskLearning.GetRiskTrend(5 * time.Minute)

	// 风险等级高且趋势增加时，应该增加延迟
	return riskLevel > 0.5 && trend == 1
}

// ShouldDecreaseDelay 判断是否应该减少延迟
//
// 返回：是否应该减少延迟
func (a *AdaptiveStrategy) ShouldDecreaseDelay() bool {
	riskLevel := a.GetRiskLevel()
	trend := a.riskLearning.GetRiskTrend(5 * time.Minute)

	// 风险等级低且趋势减少时，可以减少延迟
	return riskLevel < 0.2 && trend == -1
}

// ExtractRiskKeyword 从响应中提取风险关键词
//
// 参数：
//   - response: 响应内容
//
// 返回：风险关键词
func ExtractRiskKeyword(response string) string {
	response = strings.ToLower(response)

	// 风险关键词列表
	riskKeywords := []string{
		"验证码",
		"captcha",
		"频繁",
		"限流",
		"异常",
		"封禁",
		"锁定",
		"系统繁忙",
		"服务器忙",
	}

	// 查找匹配的关键词
	for _, keyword := range riskKeywords {
		if strings.Contains(response, keyword) {
			return keyword
		}
	}

	return ""
}

// DetectRiskPattern 检测风险模式
//
// 参数：
//   - response: 响应内容
//
// 返回：是否检测到风险模式
func DetectRiskPattern(response string) bool {
	response = strings.ToLower(response)

	// 检测 HTTP 状态码
	if strings.Contains(response, "429") || strings.Contains(response, "503") {
		return true
	}

	// 检测风险关键词
	riskKeywords := []string{
		"验证码",
		"captcha",
		"频繁",
		"限流",
		"异常",
		"封禁",
		"锁定",
		"系统繁忙",
		"服务器忙",
	}

	for _, keyword := range riskKeywords {
		if strings.Contains(response, keyword) {
			return true
		}
	}

	return false
}

// AnalyzeUserAgentConsistency 分析 User-Agent 一致性
//
// 参数：
//   - userAgent: User-Agent 字符串
//
// 返回：User-Agent 一致性分数 (0~1)
func AnalyzeUserAgentConsistency(userAgent string) float64 {
	// 真实浏览器 User-Agent 特征
	realBrowserPatterns := []string{
		"Mozilla/",
		"Chrome/",
		"Safari/",
		"Windows NT",
		"Macintosh",
		"Linux",
	}

	// 检查是否包含真实浏览器特征
	matchCount := 0
	for _, pattern := range realBrowserPatterns {
		if strings.Contains(userAgent, pattern) {
			matchCount++
		}
	}

	// 计算一致性分数
	consistency := float64(matchCount) / float64(len(realBrowserPatterns))

	return consistency
}

// DetectAnomalies 检测异常
//
// 参数：
//   - request: 请求信息
//   - history: 历史记录
//
// 返回：是否检测到异常
func DetectAnomalies(request map[string]interface{}, history []map[string]interface{}) bool {
	// 检测请求频率异常
	if len(history) > 10 {
		// 计算平均请求间隔
		avgInterval := calculateAverageInterval(history)

		// 如果当前请求间隔远小于平均值，认为是异常
		currentInterval := time.Since(request["timestamp"].(time.Time))
		if currentInterval < avgInterval/2 {
			return true
		}
	}

	return false
}

// calculateAverageInterval 计算平均请求间隔
//
// 参数：
//   - history: 历史记录
//
// 返回：平均请求间隔
func calculateAverageInterval(history []map[string]interface{}) time.Duration {
	if len(history) < 2 {
		return 0
	}

	total := time.Duration(0)
	count := 0

	for i := 1; i < len(history); i++ {
		prev := history[i-1]["timestamp"].(time.Time)
		curr := history[i]["timestamp"].(time.Time)
		interval := curr.Sub(prev)
		total += interval
		count++
	}

	if count == 0 {
		return 0
	}

	return total / time.Duration(count)
}
