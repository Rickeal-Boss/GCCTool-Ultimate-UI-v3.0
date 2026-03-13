package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gcctool/internal/model"
)

// Client HTTP客户端
type Client struct {
	httpClient *http.Client
	baseURL    string
	cookieJar  http.CookieJar
}

// NewClient 创建客户端
func NewClient(nodeURL string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * 1000000000, // 30秒
		},
		baseURL: getNodeURL(nodeURL),
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

	if url, ok := nodes[node]; ok {
		return url
	}
	return nodes["节点1（推荐）"]
}

// doGet GET请求
func (c *Client) doGet(url string) (string, error) {
	resp, err := c.httpClient.Get(url)
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

// doPost POST请求（使用url.Values确保参数顺序固定）
func (c *Client) doPost(url string, data map[string]string) (string, error) {
	// 关键修复：使用url.Values编码，确保参数顺序固定
	values := url.Values{}
	for k, v := range data {
		values.Set(k, v)
	}

	resp, err := c.httpClient.Post(url, "application/x-www-form-urlencoded", strings.NewReader(values.Encode()))
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

// doPostWithBytes POST请求（发送原始字节）
func (c *Client) doPostWithBytes(url string, data []byte, contentType string) (string, error) {
	resp, err := c.httpClient.Post(url, contentType, bytes.NewBuffer(data))
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
