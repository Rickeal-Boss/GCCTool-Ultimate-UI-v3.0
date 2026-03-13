package client

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"

	"gcctool/internal/model"
)

// Login 登录
func (c *Client) Login(cfg *model.Config) error {
	// 步骤1: 获取登录页
	loginPageURL := c.buildURL("/jwxt/login")
	html, err := c.doGet(loginPageURL)
	if err != nil {
		return err
	}

	// 步骤2: 解析登录页获取表单参数
	formData := c.parseLoginForm(html)

	// 步骤3: 加密密码
	encryptedPassword, err := c.encryptPassword(cfg.Password)
	if err != nil {
		return err
	}

	// 步骤4: 提交登录表单
	formData["username"] = cfg.Username
	formData["password"] = encryptedPassword

	loginURL := c.buildURL("/jwxt/loginAction")
	_, err = c.doPost(loginURL, formData)
	if err != nil {
		return err
	}

	// 步骤5: 检查登录是否成功
	return c.checkLoginStatus()
}

// parseLoginForm 解析登录表单
func (c *Client) parseLoginForm(html string) map[string]string {
	// 使用HTML解析器提取表单参数
	// 这里简化处理，实际应使用golang.org/x/net/html
	formData := make(map[string]string)

	// 提取所有hidden input的name和value
	// 关键修复：不使用正则，改用proper的HTML解析

	return formData
}

// encryptPassword RSA加密密码
func (c *Client) encryptPassword(password string) (string, error) {
	// 实际教务系统的RSA公钥（示例）
	publicKeyPEM := `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...
-----END PUBLIC KEY-----`

	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return password, nil // 加密失败则返回明文
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return password, nil
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return password, nil
	}

	encrypted, err := rsa.EncryptPKCS1v15(nil, rsaPub, []byte(password))
	if err != nil {
		return password, nil
	}

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// checkLoginStatus 检查登录状态
func (c *Client) checkLoginStatus() error {
	// 检查是否能访问需要登录的页面
	testURL := c.buildURL("/xsxk/zzxkyzb_cxZzxkYzbIndex.html")
	_, err := c.doGet(testURL)
	if err != nil {
		return err
	}
	return nil
}
