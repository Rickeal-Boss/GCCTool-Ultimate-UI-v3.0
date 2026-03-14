package client

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// 正方教务系统 V9（广州商学院 jwxt.gcc.edu.cn）接口路径常量
//
// 实测路径来源：https://jwxt.gcc.edu.cn 页面结构
//   - 登录页路径：  /xtgl/login_slogin.html
//   - 登录提交路径：/xtgl/login_slogin.html  (form action 与登录页同路径，POST)
//   - 选课模块：    /jwglxt/xsxk/...
// ─────────────────────────────────────────────────────────────────────────────

const (
	// 登录页路径（GET 获取公钥）
	pathLoginPage = "/xtgl/login_slogin.html"

	// 登录提交路径（POST 提交账号+加密密码，与登录页同路径）
	pathLoginPost = "/xtgl/login_slogin.html"

	// 选课首页（验证登录状态 + 获取 gnmkdm/xkkz_id 等动态参数）
	pathSelectIndex = "/jwglxt/xsxk/zzxkyzb_cxZzxkYzbIndex.html"

	// 选课参数获取
	pathSelectDisplay = "/jwglxt/xsxk/zzxkyzb_cxZzxkYzbDisplay.html"

	// 课程列表（分页）
	pathCourseList = "/jwglxt/xsxk/zzxkyzb_cxZzxkYzbPartDisplay.html"

	// 课程详情（获取 do_jxb_id 等加密 ID）
	pathCourseInfo = "/jwglxt/xsxk/zzxkyzbjk_cxJxbWithKchZzxkYzb.html"

	// 选课提交（正方 V9 路径，旧版为 _tjZzxkYzb，V9 改为此路径）
	pathSelectSubmit = "/jwglxt/xsxk/zzxkyzbjk_xkBcZyZzxkYzb.html"

	// 已选课程查询
	pathSelectedCourses = "/jwglxt/xsxk/zzxkyzb_cxYxkAndKc.html"

	// 退课
	pathCancelCourse = "/jwglxt/xsxk/zzxkyzb_tkZzxkYzb.html"

	// 功能模块代码（正方 V9 选课模块固定值，作为 URL Query 参数传递）
	gnmkdmSelect = "N253512"
)

// Login 登录广州商学院正方 V9 教务系统
//
// 流程：
//  1. GET 登录页，从 JS 变量中提取 RSA 公钥
//  2. 解析登录页所有 hidden input（含 csrftoken 等）
//  3. RSA-PKCS1v15 加密密码（加密失败严格拒绝，不降级明文）
//  4. POST 提交登录表单（参数名：yhm=学号, mm=加密密码）
//  5. 访问选课首页验证 Session 有效性
func (c *Client) Login(cfg *model.Config) error {
	// 步骤1：GET 登录页 HTML
	loginPageURL := c.buildURL(pathLoginPage)
	pageHTML, err := c.doGet(loginPageURL)
	if err != nil {
		return fmt.Errorf("获取登录页失败: %w", err)
	}

	// 步骤2：提取登录页所有 hidden input（含 csrftoken 等 CSRF 保护字段）
	formData := c.parseLoginForm(pageHTML)

	// 步骤3：从页面 JS 中动态提取 RSA 公钥并加密密码
	//
	// 正方 V9 公钥嵌入方式为 JavaScript 变量，例如：
	//   var publicKey = "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQ...";
	pubKey, err := c.extractPublicKey(pageHTML)
	if err != nil {
		return fmt.Errorf("提取RSA公钥失败，无法安全加密密码，登录已中止: %w", err)
	}

	encryptedPassword, err := encryptWithRSA(pubKey, cfg.Password)
	if err != nil {
		// 严格拒绝：加密失败时绝对不降级为明文传输
		return fmt.Errorf("RSA加密密码失败，登录已中止（拒绝明文传输）: %w", err)
	}

	// 步骤4：提交登录表单
	// 正方 V9 登录参数名：yhm=学号，mm=加密密码（V9 与旧版字段名不同）
	formData["yhm"] = cfg.Username
	formData["mm"] = encryptedPassword

	loginURL := c.buildURL(pathLoginPost)
	respBody, err := c.doPostWithReferer(loginURL, formData, loginPageURL)
	if err != nil {
		return fmt.Errorf("提交登录表单失败: %w", err)
	}

	// 步骤4.5：检查响应中是否有明确的失败提示
	if err := checkLoginResponse(respBody); err != nil {
		return err
	}

	// 步骤5：验证登录状态（访问选课首页，成功则 Session 有效）
	return c.checkLoginStatus()
}

// checkLoginResponse 检查登录响应中是否包含失败标志
func checkLoginResponse(body string) error {
	failKeywords := []string{
		"账号或密码不正确",
		"用户名或密码错误",
		"登录失败",
		"密码错误",
		"账号不存在",
		"用户名不存在",
	}
	lowerBody := strings.ToLower(body)
	for _, kw := range failKeywords {
		if strings.Contains(lowerBody, strings.ToLower(kw)) {
			return fmt.Errorf("登录失败：服务器返回错误提示「%s」", kw)
		}
	}
	return nil
}

// parseLoginForm 解析登录页 HTML，提取所有 hidden input 的 name/value
func (c *Client) parseLoginForm(pageHTML string) map[string]string {
	formData := make(map[string]string)

	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return formData
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "input" {
			attrs := attrMap(n.Attr)
			if strings.EqualFold(attrs["type"], "hidden") {
				name := attrs["name"]
				value := attrs["value"]
				if name != "" {
					formData[name] = value
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	return formData
}

// extractPublicKey 从登录页 HTML 中动态提取 RSA 公钥
//
// 正方 V9 支持三种公钥嵌入方式（按优先级顺序尝试）：
//
//  1. JavaScript 变量（V9 标准，Base64 DER 格式）：
//     var publicKey = "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQ...";
//
//  2. Hidden Input（部分旧版部署）：
//     <input type="hidden" id="publicKey" value="MIGfMA0..." />
//
//  3. 十六进制 modulus + exponent（极少数老版本）：
//     var modulus = "C497BA8F..."; var exponent = "010001";
func (c *Client) extractPublicKey(pageHTML string) (string, error) {
	// ── 方式1：JavaScript 变量（Base64 DER，最常见）──────────────────────────
	jsPatterns := []string{
		`(?i)var\s+publicKey\s*=\s*["']([A-Za-z0-9+/=]{20,})["']`,
		`(?i)publicKey\s*[=:]\s*["']([A-Za-z0-9+/=]{20,})["']`,
		`(?i)(?:rsaKey|rsa_key)\s*[=:]\s*["']([A-Za-z0-9+/=]{20,})["']`,
	}
	for _, pattern := range jsPatterns {
		re := regexp.MustCompile(pattern)
		if m := re.FindStringSubmatch(pageHTML); len(m) >= 2 {
			return m[1], nil
		}
	}

	// ── 方式2：Hidden Input──────────────────────────────────────────────────
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err == nil {
		var walk func(*html.Node) string
		walk = func(n *html.Node) string {
			if n.Type == html.ElementNode && n.Data == "input" {
				attrs := attrMap(n.Attr)
				id := strings.ToLower(attrs["id"])
				name := strings.ToLower(attrs["name"])
				if id == "publickey" || name == "publickey" || id == "rsakey" || name == "rsakey" {
					if v := attrs["value"]; v != "" {
						return v
					}
				}
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if v := walk(child); v != "" {
					return v
				}
			}
			return ""
		}
		if key := walk(doc); key != "" {
			return key, nil
		}
	}

	// ── 方式3：十六进制 modulus + exponent（极少数部署）────────────────────
	modRe := regexp.MustCompile(`(?i)var\s+modulus\s*=\s*["']([0-9A-Fa-f]{64,})["']`)
	expRe := regexp.MustCompile(`(?i)var\s+exponent\s*=\s*["']([0-9A-Fa-f]{4,})["']`)
	modMatch := modRe.FindStringSubmatch(pageHTML)
	expMatch := expRe.FindStringSubmatch(pageHTML)
	if len(modMatch) >= 2 && len(expMatch) >= 2 {
		pemStr, err := hexModExpToBase64DER(modMatch[1], expMatch[1])
		if err == nil {
			return pemStr, nil
		}
	}

	return "", fmt.Errorf("登录页中未找到RSA公钥（已尝试JS变量、hidden input、hex modulus 三种方式）")
}

// hexModExpToBase64DER 将十六进制 modulus + exponent 构造 RSA 公钥并返回 Base64 DER
func hexModExpToBase64DER(modHex, expHex string) (string, error) {
	modBytes, err := hexToBytes(modHex)
	if err != nil {
		return "", fmt.Errorf("modulus hex 解码失败: %w", err)
	}
	expBytes, err := hexToBytes(expHex)
	if err != nil {
		return "", fmt.Errorf("exponent hex 解码失败: %w", err)
	}

	n := new(big.Int).SetBytes(modBytes)
	e := new(big.Int).SetBytes(expBytes)

	rsaPub := &rsa.PublicKey{N: n, E: int(e.Int64())}
	derBytes, err := x509.MarshalPKIXPublicKey(rsaPub)
	if err != nil {
		return "", fmt.Errorf("RSA公钥序列化失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(derBytes), nil
}

// hexToBytes 将十六进制字符串转换为字节切片
func hexToBytes(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	if len(s)%2 != 0 {
		s = "0" + s
	}
	result := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		var b byte
		if _, err := fmt.Sscanf(s[i:i+2], "%02x", &b); err != nil {
			return nil, fmt.Errorf("hex 解析失败（位置 %d）: %w", i, err)
		}
		result[i/2] = b
	}
	return result, nil
}

// encryptWithRSA 使用服务端 RSA 公钥加密密码
//
// 支持两种公钥格式：
//   - 完整 PEM 格式（含 -----BEGIN PUBLIC KEY----- 头尾）
//   - 纯 Base64 DER（正方 V9 常见格式，不含 PEM 头尾）
//
// 使用 crypto/rand.Reader 作为随机源（Go 标准要求，传 nil 会 panic）。
// 加密算法：RSA-PKCS1v15（与正方前端 JSEncrypt 库兼容）。
func encryptWithRSA(pubKeyStr, password string) (string, error) {
	var derBytes []byte

	trimmed := strings.TrimSpace(pubKeyStr)

	if strings.HasPrefix(trimmed, "-----BEGIN") {
		block, _ := pem.Decode([]byte(trimmed))
		if block == nil {
			return "", fmt.Errorf("PEM解码失败")
		}
		derBytes = block.Bytes
	} else {
		var err error
		derBytes, err = base64.StdEncoding.DecodeString(trimmed)
		if err != nil {
			derBytes, err = base64.URLEncoding.DecodeString(trimmed)
			if err != nil {
				return "", fmt.Errorf("公钥Base64解码失败: %w", err)
			}
		}
	}

	// 先尝试 PKIX（SubjectPublicKeyInfo），再尝试 PKCS#1
	pub, err := x509.ParsePKIXPublicKey(derBytes)
	if err != nil {
		rsaPub, err2 := x509.ParsePKCS1PublicKey(derBytes)
		if err2 != nil {
			return "", fmt.Errorf("公钥解析失败（PKIX: %v; PKCS1: %v）", err, err2)
		}
		pub = rsaPub
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("公钥类型错误，期望 RSA，实际 %T", pub)
	}

	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, rsaPub, []byte(password))
	if err != nil {
		return "", fmt.Errorf("RSA-PKCS1v15加密失败: %w", err)
	}

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// checkLoginStatus 访问选课首页，验证 Session 是否有效
func (c *Client) checkLoginStatus() error {
	testURL := c.buildURL(pathSelectIndex) + "?gnmkdm=" + gnmkdmSelect + "&layout=default"
	body, err := c.doGet(testURL)
	if err != nil {
		return fmt.Errorf("登录状态验证失败: %w", err)
	}
	// 被重定向回登录页说明 Session 未建立
	if strings.Contains(body, "login_slogin") ||
		(strings.Contains(body, "登录") && strings.Contains(body, "密码")) {
		return fmt.Errorf("登录失败：Session 未建立，请检查账号或密码")
	}
	return nil
}

// doPostWithReferer POST 请求，支持自定义 Referer
// 登录表单提交时 Referer 应指向登录页自身，而非选课页
func (c *Client) doPostWithReferer(rawURL string, data map[string]string, referer string) (string, error) {
	values := url.Values{}
	for k, v := range data {
		values.Set(k, v)
	}

	req, err := http.NewRequest(http.MethodPost, rawURL, strings.NewReader(values.Encode()))
	if err != nil {
		return "", fmt.Errorf("构造POST请求失败: %w", err)
	}
	applyBrowserHeaders(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", referer)
	req.Header.Set("Origin", c.baseURL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("POST请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}
	return string(body), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// 内部工具函数
// ─────────────────────────────────────────────────────────────────────────────

// attrMap 将 []html.Attribute 转换为 map（key 全小写）
func attrMap(attrs []html.Attribute) map[string]string {
	m := make(map[string]string, len(attrs))
	for _, a := range attrs {
		m[strings.ToLower(a.Key)] = a.Val
	}
	return m
}
