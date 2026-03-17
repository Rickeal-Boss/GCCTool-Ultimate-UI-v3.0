// Package stealth - 风控信号检测器
//
// 正方系统已知的封号/限流信号清单（实测归纳）：
//  - 返回 HTTP 429 → 服务端明确限流
//  - 返回 HTTP 503 → 服务端过载/熔断
//  - 响应体含 "频繁" / "操作频繁" / "稍后重试" → 业务层限流
//  - 响应体含 "验证码" / "captcha" → 触发人机验证
//  - 响应体含 "账号已被锁定" / "账号异常" → 账号封禁
//  - 响应体含 "login_slogin" / "请重新登录" → Session 失效
//  - 响应体含 "系统繁忙" / "服务器忙" → 短暂降级重试
package stealth

import (
	"strings"
)

// RiskLevel 风险等级
type RiskLevel int

const (
	// RiskNone 无风险，正常继续
	RiskNone RiskLevel = iota
	// RiskRateLimit 触发限流（429/频繁），需退避重试
	RiskRateLimit
	// RiskSessionExpired Session 失效，需重新登录
	RiskSessionExpired
	// RiskCaptcha 触发验证码，停止自动化
	RiskCaptcha
	// RiskBanned 账号封禁，立刻停止
	RiskBanned
	// RiskSystemBusy 系统繁忙，短暂等待
	RiskSystemBusy
	// RiskSelectSuccess 选课成功（不是风险，但用同一套返回值体系处理）
	RiskSelectSuccess
)

// RiskSignal 风险信号检测结果
type RiskSignal struct {
	Level   RiskLevel
	Keyword string // 触发该等级的关键字
	Message string // 人类可读说明
}

// riskRules 风险规则表（按优先级排列，高危优先）
var riskRules = []struct {
	level    RiskLevel
	keywords []string
	message  string
}{
	{
		level: RiskBanned,
		keywords: []string{
			"账号已被锁定", "账号锁定", "已被锁定",
			"账号异常", "账号受限", "账号封禁",
			"暂时禁止登录", "暂停使用",
		},
		message: "账号已被封禁或锁定，请登录教务系统手动处理",
	},
	{
		level: RiskCaptcha,
		keywords: []string{
			"验证码", "captcha", "请输入验证码",
			"人机验证", "滑动验证", "点击验证",
			"verify", "imageCode", "image_code",
		},
		message: "系统触发验证码，需要人工介入",
	},
	{
		level: RiskSessionExpired,
		keywords: []string{
			"login_slogin", "请重新登录", "您已超时",
			"会话已过期", "登录超时", "重新登录",
			"未登录", "请先登录", "loginout",
			`type="password"`,
		},
		message: "Session 已失效，需要重新登录",
	},
	{
		level: RiskRateLimit,
		keywords: []string{
			"429", "操作频繁", "请求频繁", "频繁操作",
			"稍后再试", "稍后重试", "too many requests",
			"rate limit", "slow down", "限流",
			"请勿频繁", "操作过于频繁",
		},
		message: "触发频率限制，将进行退避等待",
	},
	{
		level: RiskSystemBusy,
		keywords: []string{
			"系统繁忙", "服务器繁忙", "服务器忙",
			"系统维护", "503", "service unavailable",
			"当前访问人数过多",
		},
		message: "系统繁忙，短暂等待后重试",
	},
	{
		level: RiskSelectSuccess,
		keywords: []string{
			`"flag":"1"`, `"flag": "1"`,
			"选课成功", "已成功添加",
		},
		message: "选课成功",
	},
}

// DetectRisk 检测响应体中的风险信号
//
// 按优先级从高到低扫描，返回最高等级的信号。
// 若无任何信号，返回 RiskNone。
func DetectRisk(httpStatus int, responseBody string) *RiskSignal {
	lower := strings.ToLower(responseBody)

	// HTTP 状态码快速判断
	switch httpStatus {
	case 429:
		return &RiskSignal{Level: RiskRateLimit, Keyword: "HTTP 429", Message: "服务端明确返回限流状态码"}
	case 503:
		return &RiskSignal{Level: RiskSystemBusy, Keyword: "HTTP 503", Message: "服务端过载或维护"}
	case 302, 301:
		// 重定向通常意味着 Session 失效或被踢出
		// doGet 已跟随重定向，此处 302 一般不直接出现
	}

	// 响应体关键字扫描
	for _, rule := range riskRules {
		for _, kw := range rule.keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				return &RiskSignal{
					Level:   rule.level,
					Keyword: kw,
					Message: rule.message,
				}
			}
		}
	}

	return &RiskSignal{Level: RiskNone}
}

// ShouldStop 判断是否需要立即停止（不可恢复的错误）
func (r *RiskSignal) ShouldStop() bool {
	return r.Level == RiskBanned || r.Level == RiskCaptcha
}

// ShouldReLogin 判断是否需要重新登录
func (r *RiskSignal) ShouldReLogin() bool {
	return r.Level == RiskSessionExpired
}

// ShouldBackoff 判断是否需要退避等待
func (r *RiskSignal) ShouldBackoff() bool {
	return r.Level == RiskRateLimit || r.Level == RiskSystemBusy
}

// IsSuccess 是否选课成功
func (r *RiskSignal) IsSuccess() bool {
	return r.Level == RiskSelectSuccess
}

// IsNormal 是否无风险
func (r *RiskSignal) IsNormal() bool {
	return r.Level == RiskNone
}
