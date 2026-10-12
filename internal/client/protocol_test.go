package client

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
)

// ── parseSelectResult：门次上限 / 已满回显 ──────────────────────────────────

func TestParseSelectResult_QuotaExceeded(t *testing.T) {
	c := newTestClient()
	// 实机 msg 原文：超过通识选修课本学期最高选课门次限制。
	// 这不是成功（曾照抄 Efarxs 当成成功处理 → 静默假成功），重试也无效。
	err := c.parseSelectResult(`{"flag":0,"msg":"超过通识选修课本学期最高选课门次限制"}`)
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("门次上限必须返回 ErrQuotaExceeded, got %v", err)
	}
	if err := c.parseSelectResult(`{"flag":-1,"msg":"超过门次上限"}`); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("flag=-1 + 门次文案同样必须返回 ErrQuotaExceeded, got %v", err)
	}
}

func TestParseSelectResult_FullRejectionMessage(t *testing.T) {
	c := newTestClient()
	// 实机 msg 形如 "0,<jxb_id>,<容量>," —— 已满/余量0 的数据回显，语义是"本轮未受理"
	err := c.parseSelectResult(`{"flag":-1,"msg":"0,5A901A89CE325A77E065000000000001,90,"}`)
	if !errors.Is(err, ErrSelectRetry) {
		t.Fatalf("flag=-1 必须返回 ErrSelectRetry, got %v", err)
	}
	if !strings.Contains(err.Error(), "0,5A901A89CE325A77E065000000000001,90,") {
		t.Errorf("错误信息应保留服务端原始回显: %v", err)
	}
}

// ── parseCancelResult：裸 JSON 字符串响应 ───────────────────────────────────

func TestParseCancelResult_BareStringCodes(t *testing.T) {
	c := newTestClient()
	cases := []struct {
		resp    string
		wantErr bool
	}{
		{`"1"`, false}, {"1", false}, // 成功（带引号/不带引号两种形态）
		{`"2"`, true}, {"2", true}, // 繁忙
		{`"3"`, true}, // 未知异常
		{`"4"`, true}, // 非法访问
		{`"5"`, true}, // 校验不通过
		{`"9"`, true}, // 无法识别
		{"", true},   // 空响应
	}
	for _, tc := range cases {
		err := c.parseCancelResult(tc.resp)
		if tc.wantErr && err == nil {
			t.Errorf("parseCancelResult(%q) 期望失败, got nil", tc.resp)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("parseCancelResult(%q) 期望成功, got %v", tc.resp, err)
		}
	}
	// 对象形态仍按选课解析器兼容处理
	if err := c.parseCancelResult(`{"flag":1}`); err != nil {
		t.Errorf("对象形态 flag=1 应判成功, got %v", err)
	}
}

// ── throttleSubmit：锁内限速语义 ───────────────────────────────────────────

// 教训：限速必须在锁内 sleep —— 锁外 sleep 并发调用会同时醒来，等于没限速
// （初版测试实测：并发 4 次仅间隔 301ms 而非期望 900ms）。
func TestThrottleSubmit_SerializesWithinLock(t *testing.T) {
	lastSubmitAt = time.Time{} // 重置全局状态，避免受其他测试影响

	const calls = 4
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			newTestClient().throttleSubmit()
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	want := submitMinInterval * (calls - 1)
	if elapsed < want {
		t.Fatalf("并发 %d 次提交应被串行化到 ≥%v，实际 %v（锁外 sleep 会导致同时醒来）", calls, want, elapsed)
	}
}

// ── safeRedirectPolicy：跨主机带体 POST 重定向 = 凭据外泄路径 ──────────────

func TestSafeRedirectPolicy(t *testing.T) {
	mkReq := func(method, rawURL string) *http.Request {
		req := httptest.NewRequest(method, rawURL, strings.NewReader("k=v"))
		return req
	}
	first := mkReq(http.MethodPost, "http://school.example.edu.cn/login")

	// 跨主机带体 POST → 拒绝（307/308 会把请求体原样重发到攻击者主机）
	crossHost := mkReq(http.MethodPost, "http://evil.example.com/steal")
	if err := safeRedirectPolicy(crossHost, []*http.Request{first}); err == nil {
		t.Error("跨主机带体 POST 重定向必须拒绝")
	}

	// 同主机带体 POST → 允许（≤3 跳）
	sameHost := mkReq(http.MethodPost, "http://school.example.edu.cn/login2")
	if err := safeRedirectPolicy(sameHost, []*http.Request{first}); err != nil {
		t.Errorf("同主机带体 POST 应允许: %v", err)
	}
	// 同主机超过 3 跳 → 拒绝
	via := []*http.Request{first, sameHost, sameHost, sameHost}
	if err := safeRedirectPolicy(sameHost, via); err == nil {
		t.Error("同主机重定向超过 3 跳必须拒绝")
	}

	// GET 跨主机 → 放行（CookieJar 按域隔离，无请求体可外泄）
	getReq := httptest.NewRequest(http.MethodGet, "http://evil.example.com/", nil)
	if err := safeRedirectPolicy(getReq, []*http.Request{first}); err != nil {
		t.Errorf("GET 跨主机应放行: %v", err)
	}
}

// ── 登录最小参数体 ─────────────────────────────────────────────────────────

// 实机结论（2026-09-11）：登录只收最小参数体 csrftoken+language+yhm+mm；
// 带登录页全部隐藏字段反而报"用户名或密码不正确"。
func TestLogin_MinimalBody(t *testing.T) {
	// 1024 位真实密钥（validateRSAPublicKey 下限 512 位）
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("生成测试密钥失败: %v", err)
	}

	var loginForm url.Values
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/xtgl/login_slogin.html" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`<html><form action="/xtgl/login_slogin.html" method="post">
				<input type="hidden" name="csrftoken" value="CSRF-123">
				<input type="hidden" name="language" value="zh_CN">
				<input type="hidden" name="junk_hidden" value="不应提交">
				<input type="text" name="yhm">
				<input type="password" name="mm">
			</form></html>`))
		case r.URL.Path == "/xtgl/login_getPublicKey.html":
			w.Header().Set("Content-Type", "application/json")
			n := base64.StdEncoding.EncodeToString(key.N.Bytes())
			e := base64.StdEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())
			fmt.Fprintf(w, `{"modulus":%q,"exponent":%q}`, n, e)
		case r.URL.Path == "/xtgl/login_slogin.html" && r.Method == http.MethodPost:
			_ = r.ParseForm() // PostForm 需显式解析后才可用
			loginForm = r.PostForm
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`<html><body>登录成功，正在跳转...</body></html>`))
		case r.URL.Path == "/xtgl/index_initMenu.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`<html><body><div class="nav">退出登录</div><script>index_initMenu</script></body></html>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := newChainTestClient(ts.URL)
	if err := c.Login(&model.Config{Username: "2026000001", Password: "test-pass"}); err != nil {
		t.Fatalf("Login 失败: %v", err)
	}

	if loginForm.Get("yhm") != "2026000001" {
		t.Errorf("yhm = %q", loginForm.Get("yhm"))
	}
	if loginForm.Get("mm") == "" {
		t.Error("mm（加密密码）缺失")
	}
	if loginForm.Get("csrftoken") != "CSRF-123" {
		t.Errorf("csrftoken = %q, want CSRF-123", loginForm.Get("csrftoken"))
	}
	if loginForm.Get("language") != "zh_CN" {
		t.Errorf("language = %q, want zh_CN", loginForm.Get("language"))
	}
	if _, exists := loginForm["junk_hidden"]; exists {
		t.Error("登录页其他隐藏字段不得提交（带全字段会被服务端判为密码错误）")
	}
	if len(loginForm) != 4 {
		t.Errorf("登录体应恰为 4 个字段, got %d: %v", len(loginForm), loginForm)
	}
}

// 登录失败原文"用户名或密码不正确，请重新输入！"必须被识别
// （早期关键词表只有"密码错误"，匹配不上这条真实文案）。
func TestCheckLoginResponse_RealFailureMessage(t *testing.T) {
	if err := checkLoginResponse(`<html>用户名或密码不正确，请重新输入！</html>`); err == nil {
		t.Error("真实登录失败文案必须被识别")
	}
	if err := checkLoginResponse(`<html>登录成功</html>`); err != nil {
		t.Errorf("正常响应不得误判: %v", err)
	}
}

// ── QuerySelectedCourse：端点参数白名单 + 顶层数组响应 ─────────────────────

func TestQuerySelectedCourse_WhitelistAndTopLevelArray(t *testing.T) {
	var gotForm url.Values
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "cxZzxkYzbChoosedDisplay") {
			t.Errorf("已选查询端点错误: %s（应为 cxZzxkYzbChoosedDisplay.html）", r.URL.Path)
		}
		_ = r.ParseForm() // PostForm 需显式解析后才可用
		gotForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		// 响应是顶层 JSON 数组（不是 {"tmpList":[...]}）
		w.Write([]byte(`[{"kch_id":"T0000003","kcmc":"测试课程乙","xf":"3.0","jsm":"20000001/测试教师/讲师","jsmc":"测试楼101"}]`))
	}))
	defer ts.Close()

	c := newChainTestClient(ts.URL)
	c.SetSelectInitParams(map[string]string{
		"xkkz_id": "XK", "xkxnm": "2026", "xkxqm": "1", "rlkz": "1",
		"csrftoken": "不应提交", "firstXkkzXh": "HEX",
	})

	list, err := c.QuerySelectedCourse()
	if err != nil {
		t.Fatalf("QuerySelectedCourse 失败: %v", err)
	}
	if list.Total != 1 || list.Items[0].Name != "测试课程乙" {
		t.Fatalf("顶层数组响应应能解析, got %+v", list)
	}
	if _, exists := gotForm["csrftoken"]; exists {
		t.Error("已选查询不得携带白名单外字段（整包参数池 → 服务端系统异常）")
	}
	if _, exists := gotForm["firstXkkzXh"]; exists {
		t.Error("已选查询不得携带 firstXkkzXh（白名单外）")
	}
	if gotForm.Get("xkkz_id") != "XK" {
		t.Errorf("白名单内参数应保留: xkkz_id=%q", gotForm.Get("xkkz_id"))
	}
}

func TestQuerySelectedCourse_RequiresPool(t *testing.T) {
	c := newTestClient() // 未建立参数池
	if _, err := c.QuerySelectedCourse(); err == nil {
		t.Error("未建立参数池时应给出可操作错误，而不是静默返回空列表")
	}
}
