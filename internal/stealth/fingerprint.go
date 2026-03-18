// Package stealth 提供反检测/反爬虫能力
//
// 正方教务系统常见检测手段（已实测）：
//  1. User-Agent 指纹匹配 → 使用真实 Chrome 版本轮换池
//  2. 请求频率异常（同 IP 高频）→ 随机抖动延迟 + 指数退避
//  3. 请求头缺失 / 顺序异常 → 完整浏览器头注入，顺序固定
//  4. Cookie / Session 过期 → 自动重新登录
//  5. 封号预警关键字监测 → 多层关键字扫描
//  6. 验证码跳转检测 → URL/HTML 关键字扫描
package stealth

import (
	"math/rand"
	"net/http"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// ─────────────────────────────────────────────────────────────────────────────
// User-Agent 轮换池
//
// 收录 2024-2026 最新 Chrome / Edge / Firefox 真实 UA，
// 每次请求随机抽取，降低 UA 固定被识别的概率。
// ─────────────────────────────────────────────────────────────────────────────

var userAgentPool = []string{
	// Chrome 131 - Windows 10
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	// Chrome 130 - Windows 10
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	// Chrome 129 - Windows 11
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
	// Chrome 128 - Windows 10
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
	// Edge 131 - Windows 10
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0",
	// Edge 130 - Windows 10
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.0.0",
	// Firefox 132 - Windows 10
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:132.0) Gecko/20100101 Firefox/132.0",
	// Firefox 131 - Windows 10
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:131.0) Gecko/20100101 Firefox/131.0",
	// Chrome 131 - macOS Sonoma
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	// Chrome 130 - macOS Ventura
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 13_6_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	// Safari 17 - macOS Sonoma（校园系统偶尔有 Safari 用户）
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
}

// RandomUA 从轮换池中随机选取一个 User-Agent
func RandomUA() string {
	return userAgentPool[rand.Intn(len(userAgentPool))]
}

// ─────────────────────────────────────────────────────────────────────────────
// Accept-Language 多样池
//
// 真实浏览器 Accept-Language 包含地区标签和权重，
// 使用固定值会形成可被识别的指纹。
// ─────────────────────────────────────────────────────────────────────────────

var acceptLanguagePool = []string{
	"zh-CN,zh;q=0.9,en;q=0.8",
	"zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7",
	"zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2",
	"zh,en;q=0.9",
	"zh-CN,zh;q=0.9",
}

// RandomAcceptLanguage 随机语言头
func RandomAcceptLanguage() string {
	return acceptLanguagePool[rand.Intn(len(acceptLanguagePool))]
}

// ─────────────────────────────────────────────────────────────────────────────
// 请求延迟策略
//
// 模拟真实用户的操作节奏，避免机械均匀间隔被检测到。
// ─────────────────────────────────────────────────────────────────────────────

// DelayProfile 延迟配置档位
type DelayProfile int

const (
	// DelayNormal 正常节奏（选课轮询）
	DelayNormal DelayProfile = iota
	// DelayAggressive 激进节奏（距开抢时间 < 30s 时）
	DelayAggressive
	// DelayConservative 保守节奏（检测到限流信号时）
	DelayConservative
)

// JitteredDelay 返回带随机抖动的等待时间
//
// 正态分布式抖动：均值附近波动，避免机械等待被检测。
// Normal:      200~600ms
// Aggressive:  80~250ms
// Conservative: 2000~6000ms
func JitteredDelay(profile DelayProfile) time.Duration {
	switch profile {
	case DelayAggressive:
		base := 80 + rand.Intn(80)    // 80~160ms 基础
		jitter := rand.Intn(90)        // 0~90ms 抖动
		return time.Duration(base+jitter) * time.Millisecond
	case DelayConservative:
		base := 2000 + rand.Intn(2000) // 2~4s 基础
		jitter := rand.Intn(2000)       // 0~2s 抖动
		return time.Duration(base+jitter) * time.Millisecond
	default: // DelayNormal
		base := 200 + rand.Intn(200)   // 200~400ms 基础
		jitter := rand.Intn(200)        // 0~200ms 抖动
		return time.Duration(base+jitter) * time.Millisecond
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 请求头注入策略
//
// InjectHeaders 将完整浏览器头注入 http.Request，
// 每次调用随机轮换 UA 和 Accept-Language，其余固定头保持一致。
// ─────────────────────────────────────────────────────────────────────────────

// InjectHeaders 注入浏览器请求头（随机 UA + 语言轮换）
//
// ⚠️ 移除了 Sec-Fetch-* 头，避免触发某些教务系统的反爬虫检测。
// ⚠️ Accept-Encoding 只包含 gzip, deflate，不包含 br（Brotli），
//    部分旧服务器不支持 br 并因此拒绝请求。
func InjectHeaders(req *http.Request) {
	req.Header.Set("User-Agent", RandomUA())
	req.Header.Set("Accept-Language", RandomAcceptLanguage())
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	// 不再注入 Sec-Fetch-* 头（Sec-Fetch-Mode/Sec-Fetch-Site/Sec-Fetch-Dest），
	// 某些教务系统对这些头敏感，会触发风控。
	req.Header.Set("Upgrade-Insecure-Requests", "1")
}

// InjectAJAXHeaders 注入 AJAX 风格请求头（用于 JSON 接口）
//
// ⚠️ 移除了 Sec-Fetch-* 头，避免触发某些教务系统的反爬虫检测。
// ⚠️ Accept-Encoding 只包含 gzip, deflate，不包含 br（Brotli），
//    部分旧服务器不支持 br 并因此拒绝请求。
func InjectAJAXHeaders(req *http.Request, referer string) {
	req.Header.Set("User-Agent", RandomUA())
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", RandomAcceptLanguage())
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	// 不再注入 Sec-Fetch-* 头（Sec-Fetch-Mode/Sec-Fetch-Site/Sec-Fetch-Dest），
	// 某些教务系统对这些头敏感，会触发风控。
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
}
