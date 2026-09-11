package logger

import (
	"regexp"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
// 敏感信息过滤（Sensitive Data Sanitizer）
//
// 针对教务系统场景做了专项适配：
//  1. 过滤登录凭据：密码、Token、密钥等
//  2. 过滤学生身份信息：学号、手机号、邮箱
//  3. 过滤敏感路径（防止系统路径泄露）
//
// 调用时机：processLogs 在写入 UI 和终端之前统一调用
// （此前该函数定义了却从未被任何地方调用，链路是断的）。
// ─────────────────────────────────────────────────────────────────────────────

// credentialPattern 凭据类字段
//
// 处理策略：**整体打码**，不保留任何明文字符。
// 修复：原实现对所有敏感字段一律"保留前 2 位 + 后 2 位"，
// 对密码/token 这类凭据来说等于仍然泄露 4 个字符，短口令甚至可能被大幅还原。
var credentialPattern = regexp.MustCompile(
	`(?i)\b(?:password|passwd|pwd|secret|token|auth|key|mm)\s*=\s*[^&\s\]]+`)

// identityPatterns 身份类字段
//
// 处理策略：保留首尾字符（便于人工对账），中间打码。
// 这类信息泄漏代价低于凭据，且完全打码会妨碍用户自查日志。
var identityPatterns = []*regexp.Regexp{
	// 学号 / 身份证：student_id=2021xxx、yhm=20210001（正方登录用户名字段）
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

// fullMaskPlaceholder 凭据类字段的替换文本
const fullMaskPlaceholder = "******"

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
//  1. 凭据类字段整体打码
//  2. 身份类字段保留首尾打码
//  3. 移除已知敏感路径字符串
//
// 注意：正则均为预编译，且只在日志消费端（单 goroutine）执行一次，性能影响极小。
func sanitizeLog(message string) string {
	result := message

	// 1. 凭据类：整体打码
	result = credentialPattern.ReplaceAllStringFunc(result, func(match string) string {
		eqIdx := strings.Index(match, "=")
		if eqIdx < 0 {
			return match
		}
		return match[:eqIdx+1] + fullMaskPlaceholder
	})

	// 2. 身份类：保留首尾
	for _, pattern := range identityPatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			eqIdx := strings.Index(match, "=")
			if eqIdx < 0 {
				return match
			}
			key := match[:eqIdx+1]
			val := strings.TrimSpace(match[eqIdx+1:])
			return key + maskValue(val)
		})
	}

	// 3. 敏感路径
	for _, path := range sensitivePaths {
		result = strings.ReplaceAll(result, path, "[REDACTED]")
	}

	return result
}
