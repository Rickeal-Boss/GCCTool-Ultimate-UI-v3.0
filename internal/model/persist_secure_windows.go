//go:build windows

// Package model - 密码落盘加密（Windows 实现）
//
// 使用 Windows DPAPI（CryptProtectData，CurrentUser 范围）加密密码：
//   - 密文只能在「同一台机器 + 同一 Windows 用户」下解开；
//   - 配置文件被拷贝到其他机器 / 其他用户账户后，密文无法离线还原；
//   - 防护边界：不防同一用户下运行的恶意软件（它可以直接调用同样的 API）。
//
// 落盘格式：password_v2 = "win-dpapi:<base64(DPAPI blob)>"
// 兼容读取：v1 的 Base64 伪加密（password_b64）与 plain64 方案均可解开。
package model

import (
	"encoding/base64"
	"fmt"
	"syscall"
	"unsafe"
)

var (
	crypt32DLL            = syscall.NewLazyDLL("crypt32.dll")
	procCryptProtectData   = crypt32DLL.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32DLL.NewProc("CryptUnprotectData")
	kernel32DLL            = syscall.NewLazyDLL("kernel32.dll")
	procLocalFree          = kernel32DLL.NewProc("LocalFree")
)

// dataBlob 对应 C 的 DATA_BLOB（CryptProtectData 的输入输出结构）
type dataBlob struct {
	cbData uint32
	pbData *byte
}

// sealPassword 加密密码用于落盘（Windows：DPAPI CurrentUser）
func sealPassword(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}

	buf := []byte(plain)
	in := dataBlob{cbData: uint32(len(buf)), pbData: &buf[0]}
	var out dataBlob

	// 完整 7 参数（漏掉 pPromptStruct 会导致 The parameter is incorrect）：
	// CryptProtectData(pDataIn, szDataDescr, pOptionalEntropy, pvReserved,
	//                  pPromptStruct, dwFlags /*0 = CurrentUser*/, pDataOut)
	r, _, callErr := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return "", fmt.Errorf("DPAPI CryptProtectData 失败: %w", callErr)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))

	blob := unsafe.Slice(out.pbData, out.cbData)
	return passwordSchemeWindows + ":" + base64.StdEncoding.EncodeToString(blob), nil
}

// unsealPassword 解密落盘密码
//
// 兼容三种 scheme：win-dpapi（v2 主格式）、plain64（跨平台回落）、
// 以及不带 scheme 的裸 Base64（v1 历史格式兜底）。
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
	case passwordSchemeWindows:
		blob, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return "", fmt.Errorf("DPAPI 密文 Base64 解码失败: %w", err)
		}
		if len(blob) == 0 {
			return "", nil
		}
		in := dataBlob{cbData: uint32(len(blob)), pbData: &blob[0]}
		var out dataBlob
		// 完整 7 参数：CryptUnprotectData(pDataIn, ppszDataDescr,
		//                  pOptionalEntropy, pvReserved, pPromptStruct, dwFlags, pDataOut)
		r, _, callErr := procCryptUnprotectData.Call(
			uintptr(unsafe.Pointer(&in)),
			0, 0, 0, 0, 0,
			uintptr(unsafe.Pointer(&out)),
		)
		if r == 0 {
			// 典型成因：密文来自其他机器/用户的 DPAPI（跨机器拷贝配置文件）
			return "", fmt.Errorf("DPAPI CryptUnprotectData 失败（密文可能来自其他机器或用户）: %w", callErr)
		}
		defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))

		// string([]byte) 触发拷贝——LocalFree 之后原缓冲即失效，必须拷出
		return string(unsafe.Slice(out.pbData, out.cbData)), nil

	case passwordSchemePlain64:
		b, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return "", fmt.Errorf("密码 Base64 解码失败: %w", err)
		}
		return string(b), nil

	default:
		return "", fmt.Errorf("未知的密码加密方案: %s", scheme)
	}
}
