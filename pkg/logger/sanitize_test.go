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
