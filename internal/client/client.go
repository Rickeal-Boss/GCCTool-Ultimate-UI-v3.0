package client

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/stealth"
)

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

// NodeURLFromName 将节点显示名翻译为真实 Base URL（供外部包调用）
func NodeURLFromName(name string) string {
	if u, ok := nodeURLMap[name]; ok {
		return u
	}
	return nodeURLMap["节点1（推荐）"]
}

// getNodeURL 内部使用：节点名 → Base URL
func getNodeURL(node string) string {
	return NodeURLFromName(node)
}

// Client HTTP客户端
//
// V3.1 升级：
//   - 集成 stealth 反检测引擎（随机UA、随机语言头）
//   - 集成熔断器（防止账号因高频请求被封禁）
//   - 集成退避策略（触发限流时自动降速）
//   - 请求超时从 30s 调整为 15s（快速失败，更快触发重试）
//   - 新增抢课模式标志（极致速度 + 精准风控）
type Client struct {
	httpClient *http.Client
	baseURL    string
	cookieJar  http.CookieJar

	// 反检测组件
	circuitBreaker  *stealth.CircuitBreaker
	backoffStrategy *stealth.BackoffStrategy
	delayProfile    stealth.DelayProfile

	// Speed-Opt + Anti-Fix: 抢课模式标志
	// true: 抢课阶段（极速模式 + 只检测账号封禁）
	// false: 登录/等待阶段（正常模式 + 完整风控检测）
	isRobbing bool
}

// NewClient 创建客户端
func NewClient(nodeURL string) *Client {
	jar, _ := cookiejar.New(nil)

	return &Client{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			Jar:     jar,
		},
		baseURL:   getNodeURL(nodeURL),
		cookieJar: jar,

		// 默认熔断器：5次失败开路，冷却30s
		circuitBreaker: stealth.NewCircuitBreaker("正方教务"),
		// 退避策略：5s起步，最大60s，2倍指数增长，带抖动
		backoffStrategy: stealth.NewBackoffStrategy(5*time.Second, 60*time.Second, 2.0, true),
		delayProfile:    stealth.DelayNormal,
	}
}

// NewClientWithProxy 创建带代理的客户端
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
			Timeout:   15 * time.Second,
			Jar:       jar,
			Transport: transport,
		},
		baseURL:   getNodeURL(nodeURL),
		cookieJar: jar,

		circuitBreaker:  stealth.NewCircuitBreaker("正方教务（代理）"),
		backoffStrategy: stealth.NewBackoffStrategy(5*time.Second, 60*time.Second, 2.0, true),
		delayProfile:    stealth.DelayNormal,
	}
}

// SetDelayProfile 动态调整延迟档位（外部可调用，如即将开抢时切 Aggressive）
func (c *Client) SetDelayProfile(p stealth.DelayProfile) {
	c.delayProfile = p
}

// SetRobbingMode 设置抢课模式（极致速度 + 精准风控）
//
// Speed-Opt + Anti-Fix:
//   - true: 抢课阶段（极速模式 + 只检测账号封禁）
//   - false: 登录/等待阶段（正常模式 + 完整风控检测）
func (c *Client) SetRobbingMode(enabled bool) {
	c.isRobbing = enabled
	if enabled {
		// 抢课模式：切换到极速模式
		c.delayProfile = stealth.DelayUltra
	} else {
		// 非抢课模式：恢复到正常模式
		c.delayProfile = stealth.DelayNormal
	}
}

// IsRobbingMode 检查是否处于抢课模式
func (c *Client) IsRobbingMode() bool {
	return c.isRobbing
}

// CircuitBreaker 暴露熔断器（供 robber 查询状态）
func (c *Client) CircuitBreaker() *stealth.CircuitBreaker {
	return c.circuitBreaker
}

// BackoffStrategy 暴露退避策略（供 robber 查询/重置）
func (c *Client) BackoffStrategy() *stealth.BackoffStrategy {
	return c.backoffStrategy
}

// readResponseBody 读取 HTTP 响应体，自动处理 gzip 压缩
func readResponseBody(resp *http.Response) ([]byte, error) {
	var reader io.Reader = resp.Body

	switch strings.ToLower(resp.Header.Get("Content-Encoding")) {
	case "gzip":
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			reader = resp.Body
		} else {
			defer gr.Close()
			reader = gr
		}
	}

	return io.ReadAll(reader)
}

// doGet GET请求（集成反检测引擎 + 熔断器检查）
func (c *Client) doGet(rawURL string) (string, error) {
	// 熔断器检查
	if err := c.circuitBreaker.Allow(); err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("构造GET请求失败: %w", err)
	}

	// 使用随机化请求头（反检测核心）
	stealth.InjectHeaders(req)
	// GET 请求添加 Accept（覆盖 InjectHeaders 中未设置的）
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")

	reqStart := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		return "", fmt.Errorf("GET请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := readResponseBody(resp)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	bodyStr := string(body)

	// Speed-Opt + Anti-Fix: 风控信号检测（抢课模式下只检测账号封禁）
	signal := stealth.DetectRisk(resp.StatusCode, bodyStr, c.isRobbing)

	switch {
	case signal.ShouldStop():
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodGet,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(reqStart),
			RiskLevel:  signal.Level,
			Error:      signal.Message,
			UA:         req.Header.Get("User-Agent"),
		})
		return bodyStr, fmt.Errorf("[风控-停止] %s (触发词: %s)", signal.Message, signal.Keyword)
	case signal.ShouldBackoff():
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodGet,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(reqStart),
			RiskLevel:  signal.Level,
			Error:      signal.Message,
			UA:         req.Header.Get("User-Agent"),
		})
		return bodyStr, fmt.Errorf("[风控-限流] %s (触发词: %s)", signal.Message, signal.Keyword)
	case signal.ShouldReLogin():
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodGet,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(reqStart),
			RiskLevel:  signal.Level,
			Error:      signal.Message,
			UA:         req.Header.Get("User-Agent"),
		})
		return bodyStr, fmt.Errorf("[风控-会话] %s (触发词: %s)", signal.Message, signal.Keyword)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodGet,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(reqStart),
			RiskLevel:  stealth.RiskNone,
			Error:      fmt.Sprintf("HTTP %d", resp.StatusCode),
			UA:         req.Header.Get("User-Agent"),
		})
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Request.URL.String())
	}

	c.circuitBreaker.RecordSuccess()
	c.backoffStrategy.Reset() // 成功时重置退避计时器
	stealth.Global.Record(stealth.RequestRecord{
		Timestamp:  time.Now(),
		URL:        rawURL,
		Method:     http.MethodGet,
		StatusCode: resp.StatusCode,
		Latency:    time.Since(reqStart),
		RiskLevel:  stealth.RiskNone,
		Error:      "",
		UA:         req.Header.Get("User-Agent"),
	})
	return bodyStr, nil
}

// doPost POST请求（集成反检测引擎）
func (c *Client) doPost(rawURL string, data map[string]string) (string, error) {
	// 熔断器检查
	if err := c.circuitBreaker.Allow(); err != nil {
		return "", err
	}

	values := url.Values{}
	for k, v := range data {
		values.Set(k, v)
	}

	req, err := http.NewRequest(http.MethodPost, rawURL, strings.NewReader(values.Encode()))
	if err != nil {
		return "", fmt.Errorf("构造POST请求失败: %w", err)
	}

	// AJAX 风格头（选课接口均为 AJAX 调用）
	stealth.InjectAJAXHeaders(req, c.baseURL+"/xsxk/zzxkyzb_cxZzxkYzbIndex.html")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", c.baseURL)

	reqStart := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		return "", fmt.Errorf("POST请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := readResponseBody(resp)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	bodyStr := string(body)

	// 风控信号检测
	signal := stealth.DetectRisk(resp.StatusCode, bodyStr)
	switch {
	case signal.ShouldStop():
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodPost,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(reqStart),
			RiskLevel:  signal.Level,
			Error:      signal.Message,
			UA:         req.Header.Get("User-Agent"),
		})
		return bodyStr, fmt.Errorf("[风控-停止] %s (触发词: %s)", signal.Message, signal.Keyword)
	case signal.ShouldBackoff():
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodPost,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(reqStart),
			RiskLevel:  signal.Level,
			Error:      signal.Message,
			UA:         req.Header.Get("User-Agent"),
		})
		return bodyStr, fmt.Errorf("[风控-限流] %s (触发词: %s)", signal.Message, signal.Keyword)
	case signal.ShouldReLogin():
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodPost,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(reqStart),
			RiskLevel:  signal.Level,
			Error:      signal.Message,
			UA:         req.Header.Get("User-Agent"),
		})
		return bodyStr, fmt.Errorf("[风控-会话] %s (触发词: %s)", signal.Message, signal.Keyword)
	}

	c.circuitBreaker.RecordSuccess()
	stealth.Global.Record(stealth.RequestRecord{
		Timestamp:  time.Now(),
		URL:        rawURL,
		Method:     http.MethodPost,
		StatusCode: resp.StatusCode,
		Latency:    time.Since(reqStart),
		RiskLevel:  stealth.RiskNone,
		Error:      "",
		UA:         req.Header.Get("User-Agent"),
	})
	return bodyStr, nil
}

// doPostWithBytes POST请求（发送原始字节）
func (c *Client) doPostWithBytes(rawURL string, data []byte, contentType string) (string, error) {
	if err := c.circuitBreaker.Allow(); err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, rawURL, bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("构造POST请求失败: %w", err)
	}

	stealth.InjectAJAXHeaders(req, c.baseURL+"/xsxk/zzxkyzb_cxZzxkYzbIndex.html")
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Origin", c.baseURL)

	reqStart := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodPost,
			StatusCode: 0,
			Latency:    time.Since(reqStart),
			RiskLevel:  stealth.RiskNone,
			Error:      err.Error(),
			UA:         req.Header.Get("User-Agent"),
		})
		return "", fmt.Errorf("POST请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := readResponseBody(resp)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodPost,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(reqStart),
			RiskLevel:  stealth.RiskNone,
			Error:      err.Error(),
			UA:         req.Header.Get("User-Agent"),
		})
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	bodyStr := string(body)

	// 风控信号检测
	signal := stealth.DetectRisk(resp.StatusCode, bodyStr)
	switch {
	case signal.ShouldStop():
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodPost,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(reqStart),
			RiskLevel:  signal.Level,
			Error:      signal.Message,
			UA:         req.Header.Get("User-Agent"),
		})
		return bodyStr, fmt.Errorf("[风控-停止] %s (触发词: %s)", signal.Message, signal.Keyword)
	case signal.ShouldBackoff():
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodPost,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(reqStart),
			RiskLevel:  signal.Level,
			Error:      signal.Message,
			UA:         req.Header.Get("User-Agent"),
		})
		return bodyStr, fmt.Errorf("[风控-限流] %s (触发词: %s)", signal.Message, signal.Keyword)
	case signal.ShouldReLogin():
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodPost,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(reqStart),
			RiskLevel:  signal.Level,
			Error:      signal.Message,
			UA:         req.Header.Get("User-Agent"),
		})
		return bodyStr, fmt.Errorf("[风控-会话] %s (触发词: %s)", signal.Message, signal.Keyword)
	}

	c.circuitBreaker.RecordSuccess()
	stealth.Global.Record(stealth.RequestRecord{
		Timestamp:  time.Now(),
		URL:        rawURL,
		Method:     http.MethodPost,
		StatusCode: resp.StatusCode,
		Latency:    time.Since(reqStart),
		RiskLevel:  stealth.RiskNone,
		Error:      "",
		UA:         req.Header.Get("User-Agent"),
	})
	return string(body), nil
}

// doPostWithReferer POST 请求，支持自定义 Referer（登录专用）
func (c *Client) doPostWithReferer(rawURL string, data map[string]string, referer string) (string, error) {
	// 熔断器检查
	if err := c.circuitBreaker.Allow(); err != nil {
		return "", err
	}

	values := url.Values{}
	for k, v := range data {
		values.Set(k, v)
	}

	req, err := http.NewRequest(http.MethodPost, rawURL, strings.NewReader(values.Encode()))
	if err != nil {
		return "", fmt.Errorf("构造POST请求失败: %w", err)
	}

	// 登录请求使用页面导航头（非 AJAX）
	stealth.InjectHeaders(req)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", referer)
	req.Header.Set("Origin", c.baseURL)

	reqStart := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodPost,
			StatusCode: 0,
			Latency:    time.Since(reqStart),
			RiskLevel:  stealth.RiskNone,
			Error:      err.Error(),
			UA:         req.Header.Get("User-Agent"),
		})
		return "", fmt.Errorf("POST请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := readResponseBody(resp)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodPost,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(reqStart),
			RiskLevel:  stealth.RiskNone,
			Error:      err.Error(),
			UA:         req.Header.Get("User-Agent"),
		})
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	bodyStr := string(body)

	// 风控信号检测
	signal := stealth.DetectRisk(resp.StatusCode, bodyStr)
	switch {
	case signal.ShouldStop():
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodPost,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(reqStart),
			RiskLevel:  signal.Level,
			Error:      signal.Message,
			UA:         req.Header.Get("User-Agent"),
		})
		return bodyStr, fmt.Errorf("[风控-停止] %s (触发词: %s)", signal.Message, signal.Keyword)
	case signal.ShouldBackoff():
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodPost,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(reqStart),
			RiskLevel:  signal.Level,
			Error:      signal.Message,
			UA:         req.Header.Get("User-Agent"),
		})
		return bodyStr, fmt.Errorf("[风控-限流] %s (触发词: %s)", signal.Message, signal.Keyword)
	case signal.ShouldReLogin():
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodPost,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(reqStart),
			RiskLevel:  signal.Level,
			Error:      signal.Message,
			UA:         req.Header.Get("User-Agent"),
		})
		return bodyStr, fmt.Errorf("[风控-会话] %s (触发词: %s)", signal.Message, signal.Keyword)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.circuitBreaker.RecordFailure()
		stealth.Global.Record(stealth.RequestRecord{
			Timestamp:  time.Now(),
			URL:        rawURL,
			Method:     http.MethodPost,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(reqStart),
			RiskLevel:  stealth.RiskNone,
			Error:      fmt.Sprintf("HTTP %d", resp.StatusCode),
			UA:         req.Header.Get("User-Agent"),
		})
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Request.URL.String())
	}

	c.circuitBreaker.RecordSuccess()
	stealth.Global.Record(stealth.RequestRecord{
		Timestamp:  time.Now(),
		URL:        rawURL,
		Method:     http.MethodPost,
		StatusCode: resp.StatusCode,
		Latency:    time.Since(reqStart),
		RiskLevel:  stealth.RiskNone,
		Error:      "",
		UA:         req.Header.Get("User-Agent"),
	})
	return string(body), nil
}

// CheckSessionAlive 检查 Session 是否仍然有效（保活用）
//
// 访问系统首页，若返回登录页则 Session 已失效。
func (c *Client) CheckSessionAlive() error {
	body, err := c.doGet(c.buildURL("/xtgl/index_index.html"))
	if err != nil {
		return err
	}
	if strings.Contains(body, "login_slogin") || strings.Contains(body, `type="password"`) {
		return fmt.Errorf("Session 已过期")
	}
	return nil
}

// DelayProfile 返回当前延迟档位（供 robber 读取）
func (c *Client) DelayProfile() stealth.DelayProfile {
	return c.delayProfile
}

// buildURL 构建完整URL
func (c *Client) buildURL(path string) string {
	return c.baseURL + path
}
