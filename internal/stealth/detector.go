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
//
// 参数：
//   - isRobbing: 抢课模式标志
//     - true: 抢课阶段（只检测账号封禁，其他风险忽略）
//     - false: 登录/等待阶段（完整风控检测）
//
// 修正说明（Anti-Fix-Bug）：
//   - 验证码检测已优化：区分"正常的验证码 HTML 元素"和"真正的验证码触发"
//   - 登录页 HTML 中包含 `<input name="captcha">` 等元素是正常的，不应触发风控
//   - 只有明确的"验证码触发提示"（如"请输入验证码才能继续"）才触发风控
//   - 抢课模式：只检测账号封禁（极速模式，其他风险忽略）
// 若无任何信号，返回 RiskNone。
func DetectRisk(httpStatus int, responseBody string, isRobbing bool) *RiskSignal {
	lower := strings.ToLower(responseBody)

	// Speed-Opt + Anti-Fix: 抢课模式只检测账号封禁
	if isRobbing {
		for _, rule := range riskRules {
			if rule.level == RiskBanned {
				for _, kw := range rule.keywords {
					if strings.Contains(lower, strings.ToLower(kw)) {
						return &RiskSignal{
							Level:   RiskBanned,
							Keyword: kw,
							Message: rule.message,
						}
					}
				}
			}
		}
		// 抢课模式：只检测账号封禁，其他风险全部忽略
		return &RiskSignal{Level: RiskNone}
	}

	// 正常模式：完整风控检测
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
		// 修正说明（Anti-Fix-Bug）：
		//   - 验证码检测需要特殊处理：区分"正常的验证码 HTML 元素"和"真正的验证码触发"
		//   - 登录页 HTML 中包含 `<input name="captcha">` 等元素是正常的，不应触发风控
		if rule.level == RiskCaptcha {
			// 检查是否是真正的验证码触发（而不是正常的验证码 HTML 元素）
			if !isRealCaptchaTrigger(lower) {
				continue // 不是真正的验证码触发，跳过
			}
		}

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

// isRealCaptchaTrigger 判断是否是真正的验证码触发（而不是正常的验证码 HTML 元素）
//
// 说明：
//   - 登录页 HTML 中包含 `<input name="captcha">`、`<img src="captcha.jpg">` 等元素是正常的
//   - 这些元素即使验证码当前未显示，也会存在于 HTML 中
//   - 只有明确的"验证码触发提示"才表示系统要求完成验证码
//
// 返回值：
//   - true：真正的验证码触发（需要停止）
//   - false：正常的验证码 HTML 元素（不需要停止）
func isRealCaptchaTrigger(lowerBody string) bool {
	// 1. 检查是否是正常的验证码 HTML 元素（排除误报）
	normalCaptchaPatterns := []string{
		`<input`,
		`<img`,
		`type="text"`,
		`type='text'`,
	}

	hasInputTag := false
	for _, pattern := range normalCaptchaPatterns {
		if strings.Contains(lowerBody, pattern) {
			hasInputTag = true
			break
		}
	}

	// 如果包含 input/img 标签，可能是正常的验证码 HTML 元素
	if hasInputTag {
		// 进一步检查：如果是验证码相关的 input/img，但只是表单元素，不是触发提示
		if strings.Contains(lowerBody, `name="captcha"`) || strings.Contains(lowerBody, `name='captcha'`) ||
			strings.Contains(lowerBody, `id="captcha"`) || strings.Contains(lowerBody, `id='captcha'`) {
			// 检查是否有明确的触发提示（而不是正常的表单元素）
			realTriggerPatterns := []string{
				"请输入验证码才能继续",
				"请完成人机验证",
				"验证码错误",
				"触发验证码",
				"请滑动完成验证",
				"请先完成验证码",
				"验证码不正确",
				"需要验证码",
				"系统检测到异常",
			}

			for _, pattern := range realTriggerPatterns {
				if strings.Contains(lowerBody, strings.ToLower(pattern)) {
					return true // 真正的验证码触发
				}
			}

			// 没有明确的触发提示，只是正常的验证码 HTML 元素
			return false
		}
	}

	// 2. 检查明确的验证码触发提示（即使没有 input/img 标签）
	realTriggerPatterns := []string{
		"请输入验证码才能继续",
		"请完成人机验证",
		"验证码错误",
		"触发验证码",
		"请滑动完成验证",
		"请先完成验证码",
		"验证码不正确",
		"需要验证码",
		"系统检测到异常",
	}

	for _, pattern := range realTriggerPatterns {
		if strings.Contains(lowerBody, strings.ToLower(pattern)) {
			return true // 真正的验证码触发
		}
	}

	// 既不是正常的验证码 HTML 元素，也没有明确的触发提示
	// 检查是否包含简单的"验证码"字样（可能是误报）
	if strings.Contains(lowerBody, "验证码") {
		// 简单包含"验证码"字样，但不是明确的触发提示 → 误报
		return false
	}

	return false
}

// ShouldStop 判断是否需要立即停止（不可恢复的错误）
//
// 说明：
//   - RiskBanned（账号封禁）：立即停止（无法自动恢复）
//   - RiskCaptcha（验证码）：立即停止（需要人工介入，无法自动恢复）
//
// 注意：广州商学院正方系统触发验证码是因为检测到自动化行为，
//       退避等待无法解除，必须人工处理
func (r *RiskSignal) ShouldStop() bool {
	return r.Level == RiskBanned || r.Level == RiskCaptcha
}

// ShouldReLogin 判断是否需要重新登录
func (r *RiskSignal) ShouldReLogin() bool {
	return r.Level == RiskSessionExpired
}

// ShouldBackoff 判断是否需要退避等待
//
// 说明：
//   - RiskRateLimit（限流）：退避等待
//   - RiskSystemBusy（系统繁忙）：退避等待
//
// 注意：RiskCaptcha（验证码）不会退避，而是直接停止（见 ShouldStop）
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
