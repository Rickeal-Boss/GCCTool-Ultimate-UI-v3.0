package client

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
)

// Login 登录
func (c *Client) Login(cfg *model.Config) error {
	// 步骤1: 获取登录页
	loginPageURL := c.buildURL("/jwxt/login")
	pageHTML, err := c.doGet(loginPageURL)
	if err != nil {
		return fmt.Errorf("获取登录页失败: %w", err)
	}

	// 步骤2: 解析登录页获取表单参数（含 hidden inputs）
	formData := c.parseLoginForm(pageHTML)

	// 步骤3: 从登录页动态提取 RSA 公钥并加密密码
	//
	// 大多数高校教务系统会在登录页 HTML 中嵌入当前会话的 RSA 公钥，
	// 例如：
	//   <input type="hidden" id="publicKey" value="MIIBIj..." />
	//   var publicKey = "MIIBIj...";
	// 动态提取可确保每次登录都使用服务端下发的最新公钥，
	// 彻底避免硬编码公钥过期或无效的问题。
	pubKey, err := c.extractPublicKey(pageHTML)
	if err != nil {
		return fmt.Errorf("提取RSA公钥失败，无法安全加密密码，登录已中止: %w", err)
	}

	encryptedPassword, err := encryptWithRSA(pubKey, cfg.Password)
	if err != nil {
		// 严格拒绝：加密失败时绝对不降级为明文传输
		return fmt.Errorf("RSA加密密码失败，登录已中止（拒绝明文传输）: %w", err)
	}

	// 步骤4: 提交登录表单
	formData["username"] = cfg.Username
	formData["password"] = encryptedPassword

	loginURL := c.buildURL("/jwxt/loginAction")
	_, err = c.doPost(loginURL, formData)
	if err != nil {
		return fmt.Errorf("提交登录表单失败: %w", err)
	}

	// 步骤5: 检查登录是否成功
	return c.checkLoginStatus()
}

// parseLoginForm 解析登录页 HTML，提取所有 hidden input 的 name/value
func (c *Client) parseLoginForm(pageHTML string) map[string]string {
	formData := make(map[string]string)

	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return formData
	}

	// 深度优先遍历 DOM，收集 <input type="hidden"> 的 name 和 value
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
// 兼容两种常见的嵌入方式：
//  1. <input type="hidden" id="publicKey" value="MIIBIj..." />
//  2. JavaScript 变量：var publicKey = "MIIBIj...";
//
// 提取到的是 Base64 DER 格式（不含 PEM 头尾）或完整 PEM。
func (c *Client) extractPublicKey(pageHTML string) (string, error) {
	// 方式1：hidden input，id 为 publicKey（最常见）
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

	// 方式2：JavaScript 变量 var publicKey = "..."
	jsPatterns := []string{
		`(?i)var\s+publicKey\s*=\s*["']([A-Za-z0-9+/=\-]{20,})["']`,
		`(?i)publicKey\s*[=:]\s*["']([A-Za-z0-9+/=\-]{20,})["']`,
		`(?i)rsaKey\s*[=:]\s*["']([A-Za-z0-9+/=\-]{20,})["']`,
	}
	for _, pattern := range jsPatterns {
		re := regexp.MustCompile(pattern)
		if m := re.FindStringSubmatch(pageHTML); len(m) >= 2 {
			return m[1], nil
		}
	}

	return "", fmt.Errorf("登录页中未找到RSA公钥（已尝试hidden input和JS变量两种方式）")
}

// encryptWithRSA 使用服务端 RSA 公钥加密密码
//
// 支持两种公钥格式：
//   - 完整 PEM 格式（含 -----BEGIN PUBLIC KEY----- 头尾）
//   - 纯 Base64 DER 编码（教务系统常见格式，不含 PEM 头尾）
//
// 使用 crypto/rand.Reader 作为随机源（Go 标准要求，传 nil 会 panic）。
// 加密算法：RSA-PKCS1v15（与主流教务系统前端 JSEncrypt 库兼容）。
func encryptWithRSA(pubKeyStr, password string) (string, error) {
	var derBytes []byte

	trimmed := strings.TrimSpace(pubKeyStr)

	if strings.HasPrefix(trimmed, "-----BEGIN") {
		// 完整 PEM 格式
		block, _ := pem.Decode([]byte(trimmed))
		if block == nil {
			return "", fmt.Errorf("PEM解码失败")
		}
		derBytes = block.Bytes
	} else {
		// 纯 Base64 DER（教务系统最常见格式）
		var err error
		derBytes, err = base64.StdEncoding.DecodeString(trimmed)
		if err != nil {
			// 尝试 URL-safe Base64
			derBytes, err = base64.URLEncoding.DecodeString(trimmed)
			if err != nil {
				return "", fmt.Errorf("公钥Base64解码失败: %w", err)
			}
		}
	}

	// 尝试解析 PKIX 格式（SubjectPublicKeyInfo）
	pub, err := x509.ParsePKIXPublicKey(derBytes)
	if err != nil {
		// 尝试 PKCS#1 格式（部分老系统）
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

	// 注意：必须传 crypto/rand.Reader，传 nil 会在 Go 运行时 panic
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, rsaPub, []byte(password))
	if err != nil {
		return "", fmt.Errorf("RSA-PKCS1v15加密失败: %w", err)
	}

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// checkLoginStatus 检查登录状态
func (c *Client) checkLoginStatus() error {
	testURL := c.buildURL("/xsxk/zzxkyzb_cxZzxkYzbIndex.html")
	_, err := c.doGet(testURL)
	if err != nil {
		return fmt.Errorf("登录状态验证失败: %w", err)
	}
	return nil
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
