// Package cloak - 多层伪装模块
//
// 从网络层到应用层的全方位伪装，防止被教务系统识别
package cloak

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

// FingerprintGenerator 浏览器指纹生成器
//
// 生成真实的浏览器指纹，防止被识别为自动化工具
type FingerprintGenerator struct {
	uaPool         []string
	canvasPool     []string
	webglPool      []string
	screenPool     []ScreenResolution
	timezonePool   []time.Location
	currentIndex   int
}

// ScreenResolution 屏幕分辨率
type ScreenResolution struct {
	Width  int
	Height int
	Ratio  float64
}

// NewFingerprintGenerator 创建浏览器指纹生成器
//
// 返回：浏览器指纹生成器实例
func NewFingerprintGenerator() *FingerprintGenerator {
	return &FingerprintGenerator{
		uaPool:       loadUserAgentPool(),
		canvasPool:   loadCanvasPool(),
		webglPool:    loadWebGLPool(),
		screenPool:   loadScreenPool(),
		timezonePool: loadTimezonePool(),
	}
}

// GenerateRandomUserAgent 生成随机 User-Agent
//
// 返回：随机 User-Agent
func (f *FingerprintGenerator) GenerateRandomUserAgent() string {
	return f.uaPool[rand.Intn(len(f.uaPool))]
}

// GenerateRandomCanvasFingerprint 生成随机 Canvas 指纹
//
// 返回：随机 Canvas 指纹
func (f *FingerprintGenerator) GenerateRandomCanvasFingerprint() string {
	return f.canvasPool[rand.Intn(len(f.canvasPool))]
}

// GenerateRandomWebGLFingerprint 生成随机 WebGL 指纹
//
// 返回：随机 WebGL 指纹
func (f *FingerprintGenerator) GenerateRandomWebGLFingerprint() string {
	return f.webglPool[rand.Intn(len(f.webglPool))]
}

// GenerateRandomScreenResolution 生成随机屏幕分辨率
//
// 返回：随机屏幕分辨率
func (f *FingerprintGenerator) GenerateRandomScreenResolution() ScreenResolution {
	return f.screenPool[rand.Intn(len(f.screenPool))]
}

// GenerateRandomTimezone 生成随机时区
//
// 返回：随机时区
func (f *FingerprintGenerator) GenerateRandomTimezone() *time.Location {
	return &f.timezonePool[rand.Intn(len(f.timezonePool))]
}

// GenerateCompleteFingerprint 生成完整的浏览器指纹
//
// 返回：完整的浏览器指纹（JSON 格式）
func (f *FingerprintGenerator) GenerateCompleteFingerprint() map[string]interface{} {
	screen := f.GenerateRandomScreenResolution()
	timezone := f.GenerateRandomTimezone()

	fingerprint := map[string]interface{}{
		"userAgent":      f.GenerateRandomUserAgent(),
		"canvas":         f.GenerateRandomCanvasFingerprint(),
		"webgl":          f.GenerateRandomWebGLFingerprint(),
		"screenWidth":    screen.Width,
		"screenHeight":   screen.Height,
		"screenRatio":    screen.Ratio,
		"timezone":       timezone.String(),
		"language":       "zh-CN,zh;q=0.9,en;q=0.8",
		"platform":       "Win32",
		"hardwareConcurrency": rand.Intn(8) + 2, // 2~16 核心数
		"deviceMemory":   rand.Intn(8) + 4, // 4~12 GB
	}

	return fingerprint
}

// loadUserAgentPool 加载 User-Agent 池
//
// 返回：User-Agent 池
func loadUserAgentPool() []string {
	return []string{
		// Chrome 120 (Windows)
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",

		// Chrome 120 (Mac)
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",

		// Edge 120 (Windows)
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0",

		// Firefox 121 (Windows)
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
	}
}

// loadCanvasPool 加载 Canvas 指纹池
//
// 返回：Canvas 指纹池
func loadCanvasPool() []string {
	return []string{
		"1.2.3.4.5.6.7.8.9.0.1.2.3.4.5.6.7.8.9.0.1.2.3.4.5.6.7.8.9.0.1.2.3.4.5.6.7.8.9.0.1.2.3.4.5.6.7.8.9.0",
		"2.3.4.5.6.7.8.9.0.1.2.3.4.5.6.7.8.9.0.1.2.3.4.5.6.7.8.9.0.1.2.3.4.5.6.7.8.9.0.1.2.3.4.5.6.7.8.9.0.1",
		"3.4.5.6.7.8.9.0.1.2.3.4.5.6.7.8.9.0.1.2.3.4.5.6.7.8.9.0.1.2.3.4.5.6.7.8.9.0.1.2.3.4.5.6.7.8.9.0.1.2",
	}
}

// loadWebGLPool 加载 WebGL 指纹池
//
// 返回：WebGL 指纹池
func loadWebGLPool() []string {
	return []string{
		"Intel Iris OpenGL Engine",
		"Intel UHD Graphics 620",
		"Intel HD Graphics 630",
		"NVIDIA GeForce GTX 1650",
		"AMD Radeon RX 580",
	}
}

// loadScreenPool 加载屏幕分辨率池
//
// 返回：屏幕分辨率池
func loadScreenPool() []ScreenResolution {
	return []ScreenResolution{
		{Width: 1920, Height: 1080, Ratio: 16.0 / 9.0},
		{Width: 1366, Height: 768, Ratio: 16.0 / 9.0},
		{Width: 1440, Height: 900, Ratio: 16.0 / 10.0},
		{Width: 1536, Height: 864, Ratio: 16.0 / 9.0},
		{Width: 1600, Height: 900, Ratio: 16.0 / 9.0},
		{Width: 2560, Height: 1440, Ratio: 16.0 / 9.0},
	}
}

// loadTimezonePool 加载时区池
//
// 返回：时区池
func loadTimezonePool() []time.Location {
	return []time.Location{
		*time.FixedZone("CST", 8*60*60),      // UTC+8 (中国标准时间)
		*time.FixedZone("JST", 9*60*60),      // UTC+9 (日本标准时间)
		*time.FixedZone("KST", 9*60*60),      // UTC+9 (韩国标准时间)
	}
}

// NoiseGenerator 噪声生成器
//
// 生成各种类型的噪声，用于伪装指纹
type NoiseGenerator struct {
	rand *rand.Rand
}

// NewNoiseGenerator 创建噪声生成器
//
// 返回：噪声生成器实例
func NewNoiseGenerator() *NoiseGenerator {
	return &NoiseGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// AddCanvasNoise 添加 Canvas 噪声
//
// 参数：
//   - data: Canvas 数据
//
// 返回：添加噪声后的数据
func (n *NoiseGenerator) AddCanvasNoise(data []byte) []byte {
	for i := 0; i < len(data); i += 4 {
		// 添加随机噪声（±1）
		noise := n.rand.Intn(3) - 1
		data[i] = byte(int(data[i]) + noise)
		data[i+1] = byte(int(data[i+1]) + noise)
		data[i+2] = byte(int(data[i+2]) + noise)
	}
	return data
}

// AddTimingNoise 添加时间噪声
//
// 参数：
//   - duration: 原始持续时间
//   - variance: 方差
//
// 返回：添加噪声后的持续时间
func (n *NoiseGenerator) AddTimingNoise(duration, variance time.Duration) time.Duration {
	varianceMs := int64(variance / time.Millisecond)
	randomOffset := n.rand.Int63n(2*varianceMs) - varianceMs
	return duration + time.Duration(randomOffset)*time.Millisecond
}

// AddDelayNoise 添加延迟噪声
//
// 返回：添加噪声后的延迟（100~500ms）
func (n *NoiseGenerator) AddDelayNoise() time.Duration {
	return time.Duration(100+n.rand.Intn(400)) * time.Millisecond
}

// FingerprintConsistencyChecker 指纹一致性检查器
//
// 检查浏览器指纹的一致性，防止被识别
type FingerprintConsistencyChecker struct {
	fingerprint map[string]interface{}
}

// NewFingerprintConsistencyChecker 创建指纹一致性检查器
//
// 参数：
//   - fingerprint: 浏览器指纹
//
// 返回：指纹一致性检查器实例
func NewFingerprintConsistencyChecker(fingerprint map[string]interface{}) *FingerprintConsistencyChecker {
	return &FingerprintConsistencyChecker{
		fingerprint: fingerprint,
	}
}

// CheckConsistency 检查指纹一致性
//
// 参数：
//   - currentFingerprint: 当前指纹
//
// 返回：一致性分数 (0~1)
func (f *FingerprintConsistencyChecker) CheckConsistency(currentFingerprint map[string]interface{}) float64 {
	// 检查 User-Agent 一致性
	uaConsistency := f.checkUserAgentConsistency(currentFingerprint["userAgent"].(string))

	// 检查屏幕分辨率一致性
	screenConsistency := f.checkScreenConsistency(
		currentFingerprint["screenWidth"].(int),
		currentFingerprint["screenHeight"].(int),
	)

	// 检查时区一致性
	timezoneConsistency := f.checkTimezoneConsistency(currentFingerprint["timezone"].(string))

	// 综合一致性分数
	consistency := (uaConsistency + screenConsistency + timezoneConsistency) / 3.0

	return consistency
}

// checkUserAgentConsistency 检查 User-Agent 一致性
//
// 参数：
//   - currentUA: 当前 User-Agent
//
// 返回：一致性分数 (0~1)
func (f *FingerprintConsistencyChecker) checkUserAgentConsistency(currentUA string) float64 {
	originalUA := f.fingerprint["userAgent"].(string)

	// 检查浏览器类型和版本是否一致
	originalBrowser := extractBrowserInfo(originalUA)
	currentBrowser := extractBrowserInfo(currentUA)

	if originalBrowser.Name == currentBrowser.Name {
		// 同一浏览器，检查版本
		versionDiff := abs(originalBrowser.Version - currentBrowser.Version)
		if versionDiff < 5.0 {
			return 1.0 // 版本差异小于 5，认为一致
		}
		return 0.5
	}

	return 0.0
}

// checkScreenConsistency 检查屏幕分辨率一致性
//
// 参数：
//   - currentWidth: 当前屏幕宽度
//   - currentHeight: 当前屏幕高度
//
// 返回：一致性分数 (0~1)
func (f *FingerprintConsistencyChecker) checkScreenConsistency(currentWidth, currentHeight int) float64 {
	originalWidth := f.fingerprint["screenWidth"].(int)
	originalHeight := f.fingerprint["screenHeight"].(int)

	// 检查分辨率是否完全一致
	if originalWidth == currentWidth && originalHeight == currentHeight {
		return 1.0
	}

	// 检查宽高比是否一致
	originalRatio := float64(originalWidth) / float64(originalHeight)
	currentRatio := float64(currentWidth) / float64(currentHeight)

	ratioDiff := abs(originalRatio - currentRatio)
	if ratioDiff < 0.1 {
		return 0.5 // 宽高比接近，部分一致
	}

	return 0.0
}

// checkTimezoneConsistency 检查时区一致性
//
// 参数：
//   - currentTimezone: 当前时区
//
// 返回：一致性分数 (0~1)
func (f *FingerprintConsistencyChecker) checkTimezoneConsistency(currentTimezone string) float64 {
	originalTimezone := f.fingerprint["timezone"].(string)

	if originalTimezone == currentTimezone {
		return 1.0
	}

	return 0.0
}

// BrowserInfo 浏览器信息
type BrowserInfo struct {
	Name    string
	Version float64
}

// extractBrowserInfo 提取浏览器信息
//
// 参数：
//   - userAgent: User-Agent 字符串
//
// 返回：浏览器信息
func extractBrowserInfo(userAgent string) BrowserInfo {
	// 提取浏览器名称和版本
	var browser BrowserInfo

	// Chrome
	re := regexp.MustCompile(`Chrome/(\d+\.\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(userAgent)
	if len(matches) > 1 {
		browser.Name = "Chrome"
		browser.Version = parseVersion(matches[1])
		return browser
	}

	// Firefox
	re = regexp.MustCompile(`Firefox/(\d+\.\d+)`)
	matches = re.FindStringSubmatch(userAgent)
	if len(matches) > 1 {
		browser.Name = "Firefox"
		browser.Version = parseVersion(matches[1])
		return browser
	}

	// Edge
	re = regexp.MustCompile(`Edg/(\d+\.\d+\.\d+\.\d+)`)
	matches = re.FindStringSubmatch(userAgent)
	if len(matches) > 1 {
		browser.Name = "Edge"
		browser.Version = parseVersion(matches[1])
		return browser
	}

	return browser
}

// parseVersion 解析版本号
//
// 参数：
//   - versionStr: 版本号字符串
//
// 返回：版本号（取主版本号）
func parseVersion(versionStr string) float64 {
	re := regexp.MustCompile(`^(\d+)`)
	matches := re.FindStringSubmatch(versionStr)
	if len(matches) > 1 {
		var version float64
		fmt.Sscanf(matches[1], "%f", &version)
		return version
	}
	return 0.0
}

// abs 计算绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// RotateUserAgent 轮换 User-Agent
//
// 参数：
//   - currentUA: 当前 User-Agent
//   - pool: User-Agent 池
//
// 返回：新的 User-Agent
func RotateUserAgent(currentUA string, pool []string) string {
	// 找到当前 User-Agent 的索引
	currentIndex := -1
	for i, ua := range pool {
		if ua == currentUA {
			currentIndex = i
			break
		}
	}

	// 选择下一个 User-Agent
	if currentIndex >= 0 && currentIndex < len(pool)-1 {
		return pool[currentIndex+1]
	}

	// 如果找不到或已是最后一个，随机选择
	return pool[rand.Intn(len(pool))]
}

// MaskIP 掩码 IP 地址
//
// 参数：
//   - ip: IP 地址
//   - maskCount: 掩码位数
//
// 返回：掩码后的 IP 地址
func MaskIP(ip string, maskCount int) string {
	parts := strings.Split(ip, ".")
	for i := len(parts) - maskCount; i < len(parts); i++ {
		parts[i] = "*"
	}
	return strings.Join(parts, ".")
}

// ObfuscateHeaders 混淆请求头
//
// 参数：
//   - headers: 原始请求头
//
// 返回：混淆后的请求头
func ObfuscateHeaders(headers map[string]string) map[string]string {
	obfuscated := make(map[string]string)

	// 保留关键头
	keyHeaders := []string{"User-Agent", "Accept", "Accept-Language"}
	for _, key := range keyHeaders {
		if value, ok := headers[key]; ok {
			obfuscated[key] = value
		}
	}

	// 移除可疑头
	suspiciousHeaders := []string{"X-Requested-With", "Sec-Fetch-*"}
	for _, pattern := range suspiciousHeaders {
		for key := range headers {
			if strings.Contains(key, pattern) {
				continue // 跳过可疑头
			}
		}
	}

	return obfuscated
}

// GenerateRandomSessionID 生成随机 Session ID
//
// 返回：随机 Session ID
func GenerateRandomSessionID() string {
	timestamp := time.Now().UnixNano()
	random := rand.Int63n(1000000)
	sessionID := fmt.Sprintf("%d-%d", timestamp, random)
	return base64.StdEncoding.EncodeToString([]byte(sessionID))
}
