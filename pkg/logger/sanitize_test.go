package logger

import (
	"strings"
	"testing"
)

// 脱敏函数此前定义了 76 行却从未被调用（链路是断的）。
// 这些用例既锁住"确实被接上"，也锁住分级策略：凭据整体打码、身份保留首尾。

func TestSanitizeLog_CredentialsFullyMasked(t *testing.T) {
	in := "提交 mm=Abc12345 password=Secret99 token=tok-xyz key=K123456"
	out := sanitizeLog(in)

	for _, secret := range []string{"Abc12345", "Secret99", "tok-xyz", "K123456"} {
		if strings.Contains(out, secret) {
			t.Errorf("凭据不得残留明文 %q: %s", secret, out)
		}
	}
	if !strings.Contains(out, "mm=******") {
		t.Errorf("凭据应整体打码: %s", out)
	}
}

// V3.3 补漏：csrftoken（\b 词边界导致原正则匹配不到内嵌 token）、
// 会话标识（cookie/session，泄露等同账号被劫持）、中文「密码:」格式。
func TestSanitizeLog_CsrfCookieSessionAndChinese(t *testing.T) {
	cases := []struct {
		in     string
		secret string
	}{
		{"表单携带 csrftoken=a1b2c3d4e5", "a1b2c3d4e5"},
		{"Cookie: JSESSIONID=9F2E8A71B3", "9F2E8A71B3"},
		{"session=abc123def456 已建立", "abc123def456"},
		{"登录参数 密码: MyPlainPW", "MyPlainPW"},
		{"密码=S3cret!23 提交", "S3cret!23"},
	}
	for _, c := range cases {
		out := sanitizeLog(c.in)
		if strings.Contains(out, c.secret) {
			t.Errorf("敏感值 %q 不得残留明文: %s", c.secret, out)
		}
	}
}

func TestSanitizeLog_IdentityPartiallyMasked(t *testing.T) {
	out := sanitizeLog("yhm=20210001")
	if strings.Contains(out, "20210001") {
		t.Fatalf("学号不应完整保留: %s", out)
	}
	if !strings.Contains(out, "20****01") {
		t.Errorf("身份类应保留首尾便于对账: %s", out)
	}
}

func TestSanitizeLog_LeavesNormalTextAlone(t *testing.T) {
	in := "Worker 1 尝试选课: 羽毛球 (张三)"
	if out := sanitizeLog(in); out != in {
		t.Errorf("普通日志不应被改动:\n in=%q\nout=%q", in, out)
	}

	// 典型业务文案也不应被误伤
	for _, s := range []string{
		"✓ 选课成功: 羽毛球 (张三) 周一1-2",
		"服务端返回 3 门课，但没有一门命中筛选条件",
		"⏱ 等待 1 分 30 秒...",
	} {
		if sanitizeLog(s) != s {
			t.Errorf("业务日志被误改: %q -> %q", s, sanitizeLog(s))
		}
	}
}

func TestSanitizeLog_SensitivePath(t *testing.T) {
	if out := sanitizeLog("读取 config.json 失败"); strings.Contains(out, "config.json") {
		t.Errorf("敏感路径应被屏蔽: %s", out)
	}
}

// V3.3（续审）：日志注入防护。服务端响应体会被原样写入日志（如选课结果预览、
// HTML 错误页提取文本），若其中夹带 \n / \r，会在 UI 与"复制日志"里伪造出
// 额外的假日志行、误导用户。脱敏层应把换行折叠为空格且不再残留换行符。
func TestSanitizeLog_StripsNewlines(t *testing.T) {
	evil := "选课失败: 原因\r\n[伪造] 账号已被封禁，请立即停止\r\n详情 pwd=hack123"
	out := sanitizeLog(evil)

	if strings.Contains(out, "\n") || strings.Contains(out, "\r") {
		t.Errorf("脱敏后不得再含换行符，否则可被日志注入: %q", out)
	}
	// 凭据仍应被打码
	if strings.Contains(out, "hack123") {
		t.Errorf("换行场景下凭据仍应打码: %s", out)
	}
	// 伪造行不应成为独立日志行：原换行位置应为空格
	if !strings.Contains(out, "原因 [伪造]") {
		t.Errorf("换行应被折叠为空格: %s", out)
	}
}
