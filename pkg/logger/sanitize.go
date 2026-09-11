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

// credentialPattern 凭据与会话类字段
//
// 处理策略：**整体打码**，不保留任何明文字符。
// 修复：原实现对所有敏感字段一律"保留前 2 位 + 后 2 位"，
// 对密码/token 这类凭据来说等于仍然泄露 4 个字符，短口令甚至可能被大幅还原。
//
// 覆盖范围（V3.3 安全加固）：
//   - 密码类：password / passwd / pwd / mm（正方登录表单的密码字段名）
//   - 令牌类：token / csrftoken / csrf / secret / auth / key
//     （注意 csrftoken 必须整词列出：\b 词边界使得 "token" 分支
//       匹配不到 csrftoken 中的内嵌 token）
//   - 会话类：cookie / session / jsessionid（正方 V9 为 Java 系统，
//     会话 Cookie 名固定为 JSESSIONID）—— 会话标识泄露等同于账号被劫持，
//     与密码同级处理
//   - 中文格式：密码=xxx / 密码: xxx / 口令=xxx
var credentialPattern = regexp.MustCompile(
	`(?i)(?:\b(?:password|passwd|pwd|secret|token|csrftoken|csrf|auth|key|mm|cookie|session|jsessionid)\s*=\s*[^&\s\]]+|(?:密码|口令)\s*[=:]\s*[^\s,&\]]+)`)

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

	// 1. 凭据与会话类：整体打码
	result = credentialPattern.ReplaceAllStringFunc(result, func(match string) string {
		idx := strings.IndexAny(match, "=:")
		if idx < 0 {
			return match
		}
		return match[:idx+1] + fullMaskPlaceholder
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

	// 4. 折叠换行符，防止日志注入（Log Injection）
	// 服务端响应体会被原样写入日志（如选课结果的 previewOf 预览、HTML 错误页提取文本）。
	// 若其中夹带 \n / \r，会在 UI 日志与"复制日志"里伪造出额外的假日志行，误导用户。
	// 统一替换为空格，既保留信息又不破坏单行结构。
	result = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ").Replace(result)

	return result
}
