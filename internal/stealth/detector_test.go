package stealth

import "testing"

// issue#1 复盘：V3.1 的 RiskSessionExpired 关键词里有 "重新登录"、
// "loginout"、"未登录"——它们出现在**登录后正常页面**的导航栏链接文案、
// 退出链接 href（/xtgl/loginout_loginout.html）与前端 JS 源码中，
// 导致每轮轮询拉取选课首页时都命中"会话失效"→ 触发重登录 → 循环。
// 本用例用真实页面片段锁住修复：这些元素不再触发风控。
func TestDetectRisk_LoggedInPageNotSessionExpired(t *testing.T) {
	page := `<!DOCTYPE html><html><head><title>自主选课</title></head><body>
	<div class="nav"><a href="/xtgl/loginout_loginout.html">退出</a>
	<a href="/xtgl/login_slogin.html">重新登录</a></div>
	<script>var isLogin = true; if(!isLogin){ alert("未登录"); }</script>
	<div id="xskbc">课程表加载中...</div></body></html>`

	sig := DetectRisk(200, page, false)
	if sig == nil {
		t.Fatal("DetectRisk 不应返回 nil")
	}
	if sig.Level != RiskNone {
		t.Fatalf("正常登录后页面不应触发风控，got %v（关键词 %q）", sig.Level, sig.Keyword)
	}
}

// 关键词收紧后，真正的服务端失效提示仍必须命中。
func TestDetectRisk_RealSessionExpiredStillDetected(t *testing.T) {
	cases := []string{
		"您已超时，请重新操作",
		"会话已过期",
		`{"flag":-1,"msg":"请重新登录"}`,
		"请先登录后再进行选课操作",
	}
	for _, body := range cases {
		sig := DetectRisk(200, body, false)
		if sig.Level != RiskSessionExpired {
			t.Errorf("真失效提示应命中 RiskSessionExpired: %q → %v", body, sig.Level)
		}
	}
}

// ── 2026-10-12 实机误报回归 ────────────────────────────────────────────────
// 最大教训：曾把正常页面误判成"限流/验证码"——关键词 "429" 命中的是
// right_do_jxb_id（256 位十六进制加密教学班 ID）里的数字子串；"captcha"
// 命中的是框架自带 CSS `.captcha_modal`。风控关键词绝不匹配密文/十六进制串/
// 框架类名，只认用户可读的中文提示 + HTTP 状态码。

func TestDetectRisk_HexEncryptedIDNotRateLimit(t *testing.T) {
	// 任何带已选课程的选课首页都含 256 位十六进制 right_do_jxb_id，
	// 其中可能出现 "429" 数字子串 —— 不得判为限流。
	page := `<!DOCTYPE html><html><body>
	<input type="hidden" id="right_do_jxb_id" value="65a1b1f2e68fc52c6fd429475be381f9a2c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4291">
	<div id="xskbc">课程表</div></body></html>`

	sig := DetectRisk(200, page, false)
	if sig.Level != RiskNone {
		t.Fatalf("含 429 子串的加密 ID 不得触发限流，got %v（关键词 %q）", sig.Level, sig.Keyword)
	}
	// 抢课模式同样不得误判
	if sig := DetectRisk(200, page, true); sig.Level != RiskNone {
		t.Fatalf("抢课模式下含 429 子串的加密 ID 也不得触发限流，got %v", sig.Level)
	}
}

func TestDetectRisk_FrameworkCaptchaCSSNotTrigger(t *testing.T) {
	// zftal-ui 框架每页都带 .captcha_modal 样式 —— 不得判为验证码。
	page := `<!DOCTYPE html><html><head><style>
	.captcha_modal{display:none;position:fixed;top:0;left:0;width:100%;height:100%;}
	.captcha_modal .modal-body{padding:20px;}
	</style></head><body><div class="captcha_modal"></div>
	<div id="xskbc">课程表</div></body></html>`

	sig := DetectRisk(200, page, false)
	if sig.Level == RiskCaptcha {
		t.Fatalf("框架 captcha CSS 不得触发验证码风控（关键词 %q）", sig.Keyword)
	}
	if sig.ShouldStop() {
		t.Fatal("正常页面不得触发停止自动化")
	}
}

func TestDetectRisk_RealSignalsStillDetected(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		level  RiskLevel
	}{
		{"HTTP 429 状态码", 429, "任意正文", RiskRateLimit},
		{"中文限流文案", 200, "操作过于频繁，请稍后再试", RiskRateLimit},
		{"访问过于频繁", 200, "当前访问过于频繁，请稍后重试", RiskRateLimit},
		{"账号封禁", 200, "账号已被锁定，请联系管理员", RiskBanned},
		{"真验证码-滑动", 200, "请滑动完成验证后继续选课", RiskCaptcha},
		{"真验证码-点击", 200, "请点击验证以继续操作", RiskCaptcha},
		{"真验证码-人机", 200, "请完成人机验证", RiskCaptcha},
		{"真验证码-输入", 200, "请输入验证码", RiskCaptcha},
	}
	for _, tc := range cases {
		sig := DetectRisk(tc.status, tc.body, false)
		if sig.Level != tc.level {
			t.Errorf("%s: got %v（关键词 %q）, want %v", tc.name, sig.Level, sig.Keyword, tc.level)
		}
	}
	// HTTP 429 状态码优先于正文（正文是正常页面也要判限流）
	if sig := DetectRisk(429, `{"tmpList":[]}`, false); sig.Level != RiskRateLimit {
		t.Errorf("HTTP 429 应判限流, got %v", sig.Level)
	}
	// 抢课模式下真信号同样必须检出
	if sig := DetectRisk(200, "操作过于频繁，请稍后再试", true); sig.Level != RiskRateLimit {
		t.Errorf("抢课模式下真限流文案应检出, got %v", sig.Level)
	}
	if sig := DetectRisk(200, "请滑动完成验证后继续选课", true); sig.Level != RiskCaptcha {
		t.Errorf("抢课模式下验证码提示应检出, got %v", sig.Level)
	}
}
