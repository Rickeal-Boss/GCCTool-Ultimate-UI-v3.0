//go:build !windows

// Package model - 密码落盘加密（非 Windows 回落实现）
//
// 本项目主要发布 Windows 版（CI 仅构建 Windows EXE），此文件仅为
// 类 Unix 平台的本地编译/测试提供兜底：Base64 混淆（非加密）。
// 若未来发布 Linux/macOS 版，应替换为系统钥匙串（macOS Keychain /
// libsecret），与 Windows 版的 DPAPI 保持同等强度。
package model

import (
	"encoding/base64"
	"fmt"
)

// sealPassword 加密密码用于落盘（非 Windows：Base64 混淆，非加密）
func sealPassword(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	return passwordSchemePlain64 + ":" + base64.StdEncoding.EncodeToString([]byte(plain)), nil
}

// unsealPassword 解密落盘密码
//
// plain64 可解；win-dpapi 报错（密文只能在使用 DPAPI 的原机器上解开）。
func unsealPassword(sealed string) (string, error) {
	if sealed == "" {
		return "", nil
	}

	scheme, b64, ok := splitSealed(sealed)
	if !ok {
		// v1 历史格式：整个字符串就是裸 Base64（无 scheme 前缀）
		b64, scheme = sealed, passwordSchemePlain64
	}

	switch scheme {
	case passwordSchemePlain64:
		b, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return "", fmt.Errorf("密码 Base64 解码失败: %w", err)
		}
		return string(b), nil
	case passwordSchemeWindows:
		return "", fmt.Errorf("此密码由 Windows 版本加密（DPAPI），无法在当前平台解开")
	default:
		return "", fmt.Errorf("未知的密码加密方案: %s", scheme)
	}
}
