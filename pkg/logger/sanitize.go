package logger

import (
	"regexp"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
// 敏感信息过滤（Sensitive Data Sanitizer）
//
// 集成自用户提供的"敏感信息过滤增强"方案，针对教务系统场景做了专项适配：
//  1. 过滤登录凭据：密码、Token、密钥等
//  2. 过滤学生身份信息：学号、手机号、邮箱
//  3. 过滤敏感路径（防止系统路径泄露）
//
// 调用时机：所有日志消息在写入 UI 和终端之前，统一经过此函数脱敏。
// ─────────────────────────────────────────────────────────────────────────────

// sensitivePatterns 敏感字段正则表达式（扩展版，覆盖教务系统常见场景）
var sensitivePatterns = []*regexp.Regexp{
	// 密码/Token/密钥类：password=xxx、token=xxx、mm=xxx（正方登录字段）
	regexp.MustCompile(`(?i)\b(?:password|passwd|pwd|secret|token|auth|key|mm)\s*=\s*[^&\s\]]+`),
	// 学号/身份证：student_id=2021xxx、yhm=20210001（正方登录用户名字段）
	regexp.MustCompile(`(?i)\b(?:yhm|id|身份证|学号|student_id)\s*=\s*\d{6,}`),
	// 手机号
	regexp.MustCompile(`(?i)\b(?:phone|mobile|tel)\s*=\s*\d{11}`),
	// 邮箱
	regexp.MustCompile(`(?i)\b(?:email|mail)\s*=\s*[^&\s]+@[^&\s]+\.[^&\s]+`),
}

// sensitivePaths 需要屏蔽的敏感路径片段
var sensitivePaths = []string{
	"/etc/passwd", "/etc/shadow", ".ssh/id_rsa",
	"config.json", ".env", "credentials",
}

// maskValue 脱敏单个值：保留前2位和后2位，中间替换为 ****
// 长度 ≤ 4 的值直接全部替换为 ****
func maskValue(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return value[:2] + "****" + value[len(value)-2:]
}

// sanitizeLog 对日志消息进行敏感信息脱敏处理
//
// 处理流程：
//  1. 正则匹配 key=value 格式的敏感字段，替换 value 部分
//  2. 移除已知敏感路径字符串
//
// 注意：此函数对性能影响极小（正则预编译，仅对 Info/Warn/Error/Success 消息执行一次）
func sanitizeLog(message string) string {
	result := message

	// 替换敏感字段的值
	for _, pattern := range sensitivePatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			// 找到等号位置，保留 key= 前缀，只替换 value
			eqIdx := strings.Index(match, "=")
			if eqIdx < 0 {
				return match
			}
			key := match[:eqIdx+1] // "key="
			val := strings.TrimSpace(match[eqIdx+1:])
			return key + maskValue(val)
		})
	}

	// 移除敏感路径
	for _, path := range sensitivePaths {
		result = strings.ReplaceAll(result, path, "[REDACTED]")
	}

	return result
}
