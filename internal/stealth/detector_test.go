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
