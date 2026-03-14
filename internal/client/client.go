package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// browserHeaders 统一的浏览器请求头，模拟 Chrome 131 / Windows 10
// 缺失 User-Agent、Referer、Accept 等头是自动化程序最典型的被检测特征
var browserHeaders = map[string]string{
	"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
	"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
	"Accept-Encoding": "gzip, deflate, br",
	"Cache-Control":   "no-cache",
	"Pragma":          "no-cache",
	"Connection":      "keep-alive",
}

// Client HTTP客户端
type Client struct {
	httpClient *http.Client
	baseURL    string
	cookieJar  http.CookieJar
}

// NewClient 创建客户端
//
// agentURL 为可选的 HTTP 代理地址（来自 UI 的 AgentEntry），空字符串表示不使用代理。
// CookieJar 在此初始化，确保登录后的 Session Cookie 能跨请求保持。
func NewClient(nodeURL string) *Client {
	// 初始化 CookieJar（内存存储，程序退出即清除，不写磁盘）
	jar, _ := cookiejar.New(nil)

	return &Client{
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Jar:       jar,
		},
		baseURL:   getNodeURL(nodeURL),
		cookieJar: jar,
	}
}

// NewClientWithProxy 创建带代理的客户端
//
// agentURL 格式示例：http://127.0.0.1:8080
// 代理地址仅在内存中使用，不持久化。
func NewClientWithProxy(nodeURL, agentURL string) *Client {
	jar, _ := cookiejar.New(nil)

	transport := &http.Transport{}
	if agentURL != "" {
		if proxyURL, err := url.Parse(agentURL); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	return &Client{
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Jar:       jar,
			Transport: transport,
		},
		baseURL:   getNodeURL(nodeURL),
		cookieJar: jar,
	}
}

// getNodeURL 获取节点URL
func getNodeURL(node string) string {
	nodes := map[string]string{
		"节点1（推荐）": "https://jwxt.example.com",
		"节点2（推荐）": "https://jwxt2.example.com",
		"节点3（推荐）": "https://jwxt3.example.com",
		"节点4（外网）": "https://jwxt4.example.com",
		"节点5（外网）": "https://jwxt5.example.com",
		"节点6（内网）": "http://jwxt6.example.com",
		"节点7（内网）": "http://jwxt7.example.com",
	}

	if nodeURL, ok := nodes[node]; ok {
		return nodeURL
	}
	return nodes["节点1（推荐）"]
}

// applyBrowserHeaders 将标准浏览器请求头批量注入请求
func applyBrowserHeaders(req *http.Request) {
	for k, v := range browserHeaders {
		req.Header.Set(k, v)
	}
}

// doGet GET请求（注入浏览器请求头，防止被 User-Agent 特征识别）
func (c *Client) doGet(rawURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("构造GET请求失败: %w", err)
	}
	applyBrowserHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	return string(body), nil
}

// doPost POST请求（注入浏览器请求头，包含 Referer 和 Origin 防 CSRF 校验失败）
func (c *Client) doPost(rawURL string, data map[string]string) (string, error) {
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
	// Referer 指向本站选课首页，通过教务系统的 Referer 来源检查
	req.Header.Set("Referer", c.baseURL+"/xsxk/zzxkyzb_cxZzxkYzbIndex.html")
	// Origin 标明请求来源域名，防止 CSRF 防护拒绝无来源的请求
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

// doPostWithBytes POST请求（发送原始字节，注入浏览器请求头）
func (c *Client) doPostWithBytes(rawURL string, data []byte, contentType string) (string, error) {
	req, err := http.NewRequest(http.MethodPost, rawURL, bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("构造POST请求失败: %w", err)
	}
	applyBrowserHeaders(req)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Referer", c.baseURL+"/xsxk/zzxkyzb_cxZzxkYzbIndex.html")
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

// buildURL 构建完整URL
func (c *Client) buildURL(path string) string {
	return c.baseURL + path
}
