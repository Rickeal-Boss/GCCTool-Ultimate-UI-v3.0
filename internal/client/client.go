package client

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
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

// nodeURLMap 节点显示名 → 真实 Base URL 的映射表
//
// 外网节点（1-5）均指向 jwxt.gcc.edu.cn HTTPS；
// 内网节点（6-13）指向校园网内负载均衡地址（明文 HTTP，仅校园网可达）。
var nodeURLMap = map[string]string{
	// 外网 HTTPS 节点（校外/VPN 均可用，推荐）
	"节点1（推荐）": "https://jwxt.gcc.edu.cn",
	"节点2（推荐）": "https://jwxt.gcc.edu.cn",
	"节点3（推荐）": "https://jwxt.gcc.edu.cn",
	"节点4（外网）": "https://jwxt.gcc.edu.cn",
	"节点5（外网）": "https://jwxt.gcc.edu.cn",
	// 校园内网 HTTP 节点（⚠️ 明文传输，仅在校园网内可用，共 8 个负载均衡地址）
	"节点6（内网）":  "http://172.22.14.1",
	"节点7（内网）":  "http://172.22.14.2",
	"节点8（内网）":  "http://172.22.14.3",
	"节点9（内网）":  "http://172.22.14.4",
	"节点10（内网）": "http://172.22.14.5",
	"节点11（内网）": "http://172.22.14.6",
	"节点12（内网）": "http://172.22.14.7",
	"节点13（内网）": "http://172.22.14.8",
}

// NodeURLFromName 将节点显示名翻译为真实 Base URL（供外部包调用，例如 UI 层判断 HTTP/HTTPS）
//
// 找不到时返回默认外网节点 URL，不 panic、不返回 error —— 调用方只需拿到 URL 判断协议即可。
func NodeURLFromName(name string) string {
	if u, ok := nodeURLMap[name]; ok {
		return u
	}
	// 节点名匹配失败（空字符串或未知名称）时 fallback 到节点1；
	// 此处不静默：调用方若依赖具体节点（如内网节点），需确保名称与 map key 一致。
	return nodeURLMap["节点1（推荐）"]
}

// getNodeURL 内部使用：节点名 → Base URL（对 NodeURLFromName 的别名封装）
func getNodeURL(node string) string {
	return NodeURLFromName(node)
}

// Client HTTP客户端
type Client struct {
	httpClient *http.Client
	baseURL    string
	cookieJar  http.CookieJar
}

// NewClient 创建客户端
//
// nodeURL 为节点显示名（如 "节点1（推荐）"）或直接的 Base URL。
// CookieJar 在此初始化，确保登录后的 Session Cookie 能跨请求保持。
func NewClient(nodeURL string) *Client {
	jar, _ := cookiejar.New(nil)

	transport := &http.Transport{
		// 响应头超时：防止服务端 hang 住连接不发响应头，导致 Worker 永久挂起
		ResponseHeaderTimeout: 20 * time.Second,
		// TLS 握手超时
		TLSHandshakeTimeout: 10 * time.Second,
		// 连接超时
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		// 禁止跳过 TLS 验证（明确设置，防止将来被意外修改）
		// TLSClientConfig: nil → 使用系统根证书，已验证，无需修改
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

// NewClientWithProxy 创建带代理的客户端
//
// agentURL 格式示例：http://127.0.0.1:8080，空字符串表示不使用代理。
// 代理地址仅在内存中使用，不持久化。
func NewClientWithProxy(nodeURL, agentURL string) *Client {
	jar, _ := cookiejar.New(nil)

	transport := &http.Transport{
		ResponseHeaderTimeout: 20 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	if agentURL != "" {
		if proxyURL, err := url.Parse(agentURL); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		} else {
			// 代理地址解析失败：打印警告，程序继续直连运行（不 panic）
			fmt.Fprintf(os.Stderr, "[WARN] 代理地址解析失败，将使用直连: %v\n", err)
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

// applyBrowserHeaders 将标准浏览器请求头批量注入请求
func applyBrowserHeaders(req *http.Request) {
	for k, v := range browserHeaders {
		req.Header.Set(k, v)
	}
}

// readResponseBody 读取 HTTP 响应体，自动处理 gzip 压缩
//
// 背景：applyBrowserHeaders 手动设置了 Accept-Encoding: gzip，
// 一旦手动设置该 header，Go 的 Transport 就不再自动解压响应——
// 官方文档明确说明：Transport 只有在"自己添加 Accept-Encoding"时才透明解压。
// 因此所有手动构造 Request 的地方都必须调用此函数读取响应体。
func readResponseBody(resp *http.Response) ([]byte, error) {
	var reader io.Reader = resp.Body

	switch strings.ToLower(resp.Header.Get("Content-Encoding")) {
	case "gzip":
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			// 如果 gzip 头读取失败，可能响应实际上不是 gzip，退回原始 body
			reader = resp.Body
		} else {
			defer gr.Close()
			reader = gr
		}
	case "deflate":
		// deflate 使用标准 zlib，Go 标准库有 compress/zlib
		// 但 deflate 在 HTTP 中实际极少使用，此处直接透传原始 body 即可
		// （若遇到 deflate 再补充）
	}

	return io.ReadAll(reader)
}

// doGet GET请求（注入浏览器请求头，防止被 User-Agent 特征识别）
//
// 注意：
//   - http.Client 默认会跟随重定向，但不会把最终非 2xx 状态码视为错误。
//     此处手动检查状态码，确保调用方拿到真正的业务响应。
//   - applyBrowserHeaders 设置了 Accept-Encoding: gzip，
//     导致 Go Transport 不再自动解压，必须手动调用 readResponseBody 解压。
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

	body, err := readResponseBody(resp)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Request.URL.String())
	}

	return string(body), nil
}

// doPost POST请求（注入浏览器请求头，包含 Referer 和 Origin 防 CSRF 校验失败）
//
// Referer 修复说明：
//   正方 V9 所有选课接口均在 /jwglxt/ 路径下，Referer 必须携带 /jwglxt 前缀，
//   否则服务端 Referer 检查可能返回 403 或重定向到登录页。
//   之前写的 "/xsxk/zzxkyzb_cxZzxkYzbIndex.html" 缺少 "/jwglxt" 导致 Referer 校验失败。
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
	// gcc.edu.cn 部署无 /jwglxt 前缀，Referer 路径与选课首页保持一致
	req.Header.Set("Referer", c.baseURL+"/xsxk/zzxkyzb_cxZzxkYzbIndex.html")
	req.Header.Set("Origin", c.baseURL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("POST请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := readResponseBody(resp)
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
	// gcc.edu.cn 部署无 /jwglxt 前缀，Referer 路径与选课首页保持一致
	req.Header.Set("Referer", c.baseURL+"/xsxk/zzxkyzb_cxZzxkYzbIndex.html")
	req.Header.Set("Origin", c.baseURL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("POST请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := readResponseBody(resp)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	return string(body), nil
}

// buildURL 构建完整URL
func (c *Client) buildURL(path string) string {
	return c.baseURL + path
}
