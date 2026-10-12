// Package stealth - 风控信号检测器
//
// 正方系统已知的封号/限流信号清单（实测归纳）：
//   - 返回 HTTP 429 → 服务端明确限流
//   - 返回 HTTP 503 → 服务端过载/熔断
//   - 响应体含 "操作频繁" / "访问过于频繁" 等用户可读文案 → 业务层限流
//   - 响应体含用户可读的验证码提示（如"请滑动完成验证"）→ 触发人机验证
//   - 响应体含 "账号已被锁定" / "账号异常" → 账号封禁
//   - 响应体含 "请重新登录" / "会话已过期" → Session 失效
//   - 响应体含 "系统繁忙" / "服务器忙" → 短暂降级重试
//
// ⚠️ 2026-10-12 实机误报修复（最大教训）：风控关键词**绝不匹配密文/十六进制串/
// 框架类名** —— "429" 会命中 right_do_jxb_id（256 位十六进制加密 ID）里的数字
// 子串（任何带已选课程的页面都有），"captcha" 会命中框架自带 CSS
// `.captcha_modal`（每页都有）。只认用户可读的中文提示 + HTTP 状态码。
// （曾把正常页面误判成限流，连错 25 分钟以上；回归测试见 detector_test.go）
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
		// 2026-10-12 实机误报修复：原关键词表里的 "captcha"/"verify"/"imageCode"
		// 命中的是框架自带 CSS `.captcha_modal{...}`（zftal-ui，每页都有）——
		// 正常页面会被判成验证码并**直接停止自动化**。
		// 风控关键词绝不匹配密文/十六进制串/框架类名，只认用户可读的中文提示。
		keywords: []string{
			"请输入验证码", "请完成人机验证", "验证码错误", "验证码不正确",
			"触发验证码", "请滑动完成验证", "滑动验证", "点击验证",
			"人机验证", "请先完成验证码", "需要验证码", "系统检测到异常",
		},
		message: "系统触发验证码，需要人工介入",
	},
	{
		level: RiskSessionExpired,
		keywords: []string{
			// Anti-Fix-Bug: 只保留服务端渲染的失效提示文案（正常登录后的页面不含这些词）
			// V3.3（issue#1 复盘）移除三个必然误报的词：
			//   - "重新登录"：登录后页面的导航栏常见"重新登录"链接文案，每轮轮询必命中
			//   - "loginout"：正方导航栏"退出"链接 href 为 /xtgl/loginout_loginout.html，
			//     任何完整 HTML 页面（选课首页等）都含此片段 → 每次拉列表必判会话失效
			//     → 触发重登录循环（issue#1 的主因）
			//   - "未登录"：选课页前端 JS 源码（变量名/提示分支）可能包含
			// 真正的会话失效由两处确定性信号兜底：
			//   - AJAX 链路：doPost/doPostWithBytes 的 looksLikeLoginPage（302→登录页）
			//   - 页面链路：doGet 同样接入 looksLikeLoginPage
			"请重新登录", "您已超时",
			"会话已过期", "登录超时", "请先登录",
		},
		message: "Session 已失效，需要重新登录",
	},
	{
		level: RiskRateLimit,
		keywords: []string{
			// Anti-Fix-Bug: 移除 "稍后再试"、"稍后重试" 等过于敏感的关键词
			// 这些词可能出现在普通的登录失败提示、帮助信息中，导致误判
			// 只保留明确的频率限制提示
			//
			// 2026-10-12 实机误报修复：移除裸 "429" —— 它命中的是选课首页
			// right_do_jxb_id（256 位十六进制**加密**教学班 ID）里的数字子串
			// （如 `65a1b1f2e68fc52c6fd429475be381f9…`），任何带已选课程的页面
			// 都有 → 每轮拉列表必被判限流。真限流由 HTTP 状态码 429 分支
			// + 中文用户可读文案捕获。
			"操作频繁", "请求频繁", "频繁操作",
			"too many requests", "rate limit", "slow down", "限流",
			"请勿频繁", "操作过于频繁", "访问过于频繁",
		},
		message: "触发频率限制，将进行退避等待",
	},
	{
		level: RiskSystemBusy,
		keywords: []string{
			// Anti-Fix-Bug: 移除 "503"，HTTP 503 应该在状态码检测中处理，不应该作为响应体关键词
			// 广州商学院教务系统可能返回包含 "503" 的错误页，但这不代表系统繁忙
			// HTTP 503 应该通过重试机制处理，而不是触发风控
			"系统繁忙", "服务器繁忙", "服务器忙",
			"系统维护", "service unavailable",
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
//     - true: 抢课阶段（保留封禁 / Session 失效 / 明确限流三类致命信号）
//     - false: 登录/等待阶段（完整风控检测）
//
// 修正说明（Anti-Fix-Bug）：
//   - 验证码检测已优化：区分"正常的验证码 HTML 元素"和"真正的验证码触发"
//   - 登录页 HTML 中包含 `<input name="captcha">` 等元素是正常的，不应触发风控
//   - 只有明确的"验证码触发提示"（如"请输入验证码才能继续"）才触发风控
//   - 抢课模式：保留封禁/Session失效/429 限流，跳过验证码与系统繁忙扫描
// 若无任何信号，返回 RiskNone。
func DetectRisk(httpStatus int, responseBody string, isRobbing bool) *RiskSignal {
	lower := strings.ToLower(responseBody)

	// 抢课模式：保留"账号封禁 / Session 失效 / 明确限流"三类致命信号
	//
	// 修复（V3.0 bug）：原实现抢课模式下只检测账号封禁，其余一律返回 RiskNone，
	// 于是 429 限流与 Session 失效在开抢后完全收不到信号 —— 恰恰在最需要检测的
	// 阶段（多线程 × 毫秒级间隔）彻底失明，会一路对着限流继续猛刷。
	// 现在保留三类必须响应的信号，同时跳过验证码/系统繁忙的正文扫描以维持速度。
	if isRobbing {
		// HTTP 429 是最明确的限流信号
		if httpStatus == 429 {
			return &RiskSignal{Level: RiskRateLimit, Keyword: "HTTP 429", Message: "服务端明确返回限流状态码"}
		}

		// 明确的限流文案（与完整规则表对齐的低误报子集；
		// ⚠️ "操作过于频繁" 不是 "操作频繁" 的子串，必须逐条列出）
		for _, kw := range []string{"操作过于频繁", "操作频繁", "访问过于频繁", "请勿频繁", "请求频繁", "频繁操作"} {
			if strings.Contains(lower, kw) {
				return &RiskSignal{Level: RiskRateLimit, Keyword: kw, Message: "触发频率限制，将进行退避等待"}
			}
		}

		// 账号封禁 / Session 失效 / 验证码 / 系统繁忙：复用完整规则表
		// （这四类不处理会直接导致空转、封号或对着验证码猛刷）。
		// 修正：V3.x 原实现在抢课模式下跳过验证码与系统繁忙扫描"以维持速度"——
		// 风控信号本就来自已经拿到的响应体，多做几次 strings.Contains 成本可忽略，
		// 而漏掉"验证码停止信号"的后果是自动化继续撞墙、加速封号。
		for _, rule := range riskRules {
			switch rule.level {
			case RiskBanned, RiskSessionExpired, RiskSystemBusy:
				// 直接扫关键词表
			case RiskCaptcha:
				// 先过真实触发门槛（排除框架 CSS/表单元素）
				if !isRealCaptchaTrigger(lower) {
					continue
				}
			default:
				continue
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

	// 正常模式：完整风控检测
	// HTTP 状态码快速判断
	switch httpStatus {
	case 429:
		return &RiskSignal{Level: RiskRateLimit, Keyword: "HTTP 429", Message: "服务端明确返回限流状态码"}
	case 503:
		// Anti-Fix-Bug: HTTP 503 不应该触发风控，应该通过重试机制处理
		// 503 可能只是临时错误（网络问题、服务器重启等），不代表系统繁忙
		// 返回 RiskNone 让上层重试机制处理
		return &RiskSignal{Level: RiskNone, Keyword: "", Message: ""}
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

// realCaptchaTriggers 真正的验证码触发文案（用户可读的中文提示）。
//
// 2026-10-12 实机误报修复的两个要点：
//  1. **框架类名不是信号**：zftal-ui 每页都带 `<style>.captcha_modal{…}</style>`，
//     匹配 "captcha" 会把正常页面判成验证码并直接停止自动化；
//  2. **门槛漏洞**：原实现里"滑动验证/点击验证/人机验证"在规则表 keywords 里，
//     却不在 isRealCaptchaTrigger 的触发词表里 → 真正的验证码永远打不出来。
//     现在两边共用同一份触发词表，消除不一致。
var realCaptchaTriggers = []string{
	"请输入验证码", "请完成人机验证", "验证码错误", "验证码不正确",
	"触发验证码", "请滑动完成验证", "滑动验证", "点击验证",
	"人机验证", "请先完成验证码", "需要验证码", "系统检测到异常",
}

// isRealCaptchaTrigger 判断是否是真正的验证码触发（而不是正常的验证码 HTML 元素/CSS）
//
// 只认服务端渲染给用户看的中文提示文案；登录页自带的 `<input name="captcha">`、
// 框架 `.captcha_modal` 样式、十六进制串都不算。
func isRealCaptchaTrigger(lowerBody string) bool {
	for _, pattern := range realCaptchaTriggers {
		if strings.Contains(lowerBody, pattern) {
			return true
		}
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
