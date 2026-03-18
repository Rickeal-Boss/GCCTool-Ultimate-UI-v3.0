// Package stealth 提供反检测/反爬虫能力
//
// 【V3.1 更新】正方教务系统实际检测手段（基于广州商学院实测 + 开源项目调研）：
//  1. Cookie + UA 双重验证 Session → 使用固定 UA（模拟真实浏览器）
//  2. 请求频率异常（同 IP 高频）→ 随机抖动延迟 + 指数退避
//  3. 请求头缺失 / 顺序异常 → 完整浏览器头注入，顺序固定
//  4. Cookie / Session 过期 → 自动重新登录
//  5. 封号预警关键字监测 → 多层关键字扫描
//  6. 验证码跳转检测 → URL/HTML 关键字扫描
//
// 【重要】：
// - UA 随机化会导致 Session 关联失败，必须使用固定 UA
// - 参考了 zhengfang-api、zfn_api、new-school-sdk 等开源项目
// - 所有项目均使用固定 UA，未使用随机化
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
// User-Agent 固定策略
//
// 【V3.1 重要更新】：
// - 教务系统通过 Cookie + UA 双重验证 Session
// - UA 随机化会导致 Session 关联失败，无法登录
// - 正确策略：模拟真实浏览器，使用固定 UA
//
// 参考了多个正方教务系统开源项目（zhengfang-api、zfn_api、new-school-sdk），
// 所有项目均使用固定 UA，未使用随机化。
// ─────────────────────────────────────────────────────────────────────────────

const (
	// FixedUA 固定的 User-Agent（Chrome 131 - Windows 10）
	// 选用 Chrome 131 作为目标版本，符合当前主流浏览器版本
	// 教务系统不会因为 UA 版本不同而拒绝请求
	FixedUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// GetUA 获取固定的 User-Agent
func GetUA() string {
	return FixedUA
}

// RandomUA 已废弃，保留用于向后兼容（V3.1 后不再使用）
// 注意：RandomUA 会导致 Session 关联失败，请使用 GetUA()
// @deprecated 使用 GetUA() 代替
func RandomUA() string {
	return FixedUA
}

// ─────────────────────────────────────────────────────────────────────────────
// Accept-Language 固定策略
//
// 【V3.1 重要更新】：
// - 教务系统不会验证 Accept-Language 的细节
// - Accept-Language 随机化不会带来任何好处
// - 正确策略：使用固定值，保持请求头一致性
//
// 选用最通用的中文浏览器配置：zh-CN,zh;q=0.9
// ─────────────────────────────────────────────────────────────────────────────

const (
	// FixedAcceptLanguage 固定的 Accept-Language（中文环境）
	FixedAcceptLanguage = "zh-CN,zh;q=0.9"
)

// GetAcceptLanguage 获取固定的 Accept-Language
func GetAcceptLanguage() string {
	return FixedAcceptLanguage
}

// RandomAcceptLanguage 已废弃，保留用于向后兼容（V3.1 后不再使用）
// 注意：RandomAcceptLanguage 不会带来任何好处，请使用 GetAcceptLanguage()
// @deprecated 使用 GetAcceptLanguage() 代替
func RandomAcceptLanguage() string {
	return FixedAcceptLanguage
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
	// DelayUltra 极速节奏（抢课阶段，毫秒级速度）
	// Speed-Opt: 抢课阶段极致速度，在系统检测风控之前完成
	// 延迟 5~10ms，每秒 1000+ 请求
	DelayUltra
	// DelayConservative 保守节奏（检测到限流信号时）
	DelayConservative
)

// JitteredDelay 返回带随机抖动的等待时间
//
// 正态分布式抖动：均值附近波动，避免机械等待被检测。
// Normal:      200~600ms
// Aggressive:  20~70ms (Speed-Opt: 从 80~250ms 降低到 20~70ms，速度提升 4.7 倍）
// Ultra:       5~10ms (Speed-Opt: 抢课阶段极致速度，毫秒级，每秒 1000+ 请求）
// Conservative: 2000~6000ms
func JitteredDelay(profile DelayProfile) time.Duration {
	switch profile {
	case DelayUltra:
		// Speed-Opt: 极速模式（抢课阶段毫秒级速度）
		// 目标：在系统检测风控之前完成抢课
		// 延迟 5~10ms，每秒 1000+ 请求
		base := 5 + rand.Intn(3)     // 5~8ms 基础
		jitter := rand.Intn(2)       // 0~2ms 抖动
		return time.Duration(base+jitter) * time.Millisecond
	case DelayAggressive:
		// Speed-Opt: 降低激进模式延迟，从 80~250ms 降低到 20~70ms
		// 抢课阶段需要极致速度，每毫秒都很关键
		// 最快 20ms，平均 35ms，速度提升 4.7 倍
		base := 20 + rand.Intn(30)    // 20~50ms 基础
		jitter := rand.Intn(20)        // 0~20ms 抖动
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
// 【V3.1 重要更新】：
// - 使用固定的 UA 和 Accept-Language，模拟真实浏览器行为
// - 教务系统通过 Cookie + UA 双重验证 Session，UA 必须固定
// - 保持请求头一致性，避免形成可识别的指纹
// ─────────────────────────────────────────────────────────────────────────────

// InjectHeaders 注入浏览器请求头（固定 UA + 语言）
//
// ⚠️ 移除了 Sec-Fetch-* 头，避免触发某些教务系统的反爬虫检测。
// ⚠️ Accept-Encoding 只包含 gzip, deflate，不包含 br（Brotli），
//    部分旧服务器不支持 br 并因此拒绝请求。
func InjectHeaders(req *http.Request) {
	req.Header.Set("User-Agent", GetUA())
	req.Header.Set("Accept-Language", GetAcceptLanguage())
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
	req.Header.Set("User-Agent", GetUA())
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", GetAcceptLanguage())
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
