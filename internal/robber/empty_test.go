package robber

import (
	"strings"
	"testing"
)

func TestEmptyResultErrorMessages(t *testing.T) {
	cases := []struct {
		name string
		err  *EmptyResultError
		want string
	}{
		{"服务端 0 门", &EmptyResultError{Kind: EmptyNoCourseAtServer}, "0 门课程"},
		{"筛选全不命中", &EmptyResultError{Kind: EmptyFilteredOut, Total: 5}, "没有一门命中筛选条件"},
		{"候选全满员", &EmptyResultError{Kind: EmptyAllFull, Total: 5, Matched: 3}, "全部满员"},
	}
	for _, c := range cases {
		if got := c.err.Error(); !strings.Contains(got, c.want) {
			t.Errorf("%s: %q 不包含 %q", c.name, got, c.want)
		}
	}

	// Total 应出现在"筛选全不命中"的文案里，便于用户判断该放宽还是该换类别
	msg := (&EmptyResultError{Kind: EmptyFilteredOut, Total: 42}).Error()
	if !strings.Contains(msg, "42") {
		t.Errorf("应带上服务端返回的课程数: %q", msg)
	}
}

// 客户端对「账号封禁」与「验证码」都返回 "[风控-停止]" 前缀，
// 必须靠 isCaptchaError 先分流，否则触发验证码时会误报"账号已被封禁"，
// 用户会以为账号出事而白白放弃抢课窗口。
func TestCaptchaIsNotTreatedAsBan(t *testing.T) {
	captcha := "[风控-停止] 系统触发验证码，需要人工介入 (触发词: 验证码)"

	if !isCaptchaError(captcha) {
		t.Fatal("验证码信号应被 isCaptchaError 识别")
	}
	// 前提：该串确实也会命中封禁判定 —— 所以 switch 里的顺序至关重要
	if !isBannedError(captcha) {
		t.Fatal("前提不成立：该串应同时命中 isBannedError")
	}

	ban := "[风控-停止] 账号已被封禁或锁定 (触发词: 账号已被锁定)"
	if isCaptchaError(ban) {
		t.Error("封禁信号不应被误判为验证码")
	}
	if !isBannedError(ban) {
		t.Error("封禁信号应被 isBannedError 识别")
	}

	// captcha 的英文关键词
	if !isCaptchaError("captcha required") {
		t.Error("captcha 关键词应被识别")
	}
}

func TestSessionErrorClassification(t *testing.T) {
	yes := []string{
		"[风控-会话] 会话已失效（接口返回了登录页）",
		"[风控-会话] Session已失效，服务器重定向到登录页(302)",
	}
	for _, s := range yes {
		if !isSessionError(s) {
			t.Errorf("应判定为会话失效: %q", s)
		}
	}

	// 选课未开放属于正常业务状态，绝不能被当成会话失效（否则会疯狂重登录）
	no := []string{
		"选课未开放：当前不在选课阶段，请等待系统开放后再试",
		"系统维护：教务系统正在维护，请稍后再试",
		"教务系统当前查询条件下返回 0 门课程",
	}
	for _, s := range no {
		if isSessionError(s) {
			t.Errorf("不应判定为会话失效: %q", s)
		}
	}
}

func TestRateLimitErrorClassification(t *testing.T) {
	yes := []string{"[风控-限流] 触发频率限制，将进行退避等待", "HTTP 429: ...", "too many requests"}
	for _, s := range yes {
		if !isRateLimitError(s) {
			t.Errorf("应判定为限流: %q", s)
		}
	}
	no := []string{
		"教务系统当前查询条件下返回 0 门课程（接口正常，本类别可能未排课）",
		"本轮未抢到课程，继续轮询",
	}
	for _, s := range no {
		if isRateLimitError(s) {
			t.Errorf("不应判定为限流: %q", s)
		}
	}
}

func TestRoundNoSelectionIsNotFatal(t *testing.T) {
	// errRoundNoSelection 走 Info 级，而不是被当成致命错误
	if errRoundNoSelection == nil {
		t.Fatal("errRoundNoSelection 不应为 nil")
	}
	msg := errRoundNoSelection.Error()
	if isBannedError(msg) || isSessionError(msg) || isRateLimitError(msg) || isCaptchaError(msg) {
		t.Errorf("单轮未选上不应被归类为风控/致命错误: %q", msg)
	}
}
