package model

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSealOpenPasswordRoundTrip 加密 → 解密 必须还原出原文
func TestSealOpenPasswordRoundTrip(t *testing.T) {
	cases := []string{"", "p", "MyP@ssw0rd-2026", "中文密码测试", strings.Repeat("x", 256)}
	for _, plain := range cases {
		sealed, err := sealPassword(plain)
		if err != nil {
			t.Fatalf("sealPassword(%q) 意外失败: %v", plain, err)
		}
		if plain != "" {
			// 密文不能是原文的直接 Base64（那等于没加密）
			if sealed == base64.StdEncoding.EncodeToString([]byte(plain)) {
				t.Fatalf("sealPassword(%q) 仅做了 Base64 编码，未加密", plain)
			}
			if !strings.Contains(sealed, ":") {
				t.Fatalf("sealPassword(%q) 缺少 scheme 前缀: %q", plain, sealed)
			}
		}
		got, err := unsealPassword(sealed)
		if err != nil {
			t.Fatalf("unsealPassword(%q) 失败: %v", sealed, err)
		}
		if got != plain {
			t.Fatalf("round-trip 不一致: got %q, want %q", got, plain)
		}
	}
}

// TestUnsealGarbageFails 损坏密文必须报错（而不是返回空成功），
// 否则跨机器拷贝的 DPAPI 密文会被静默当成"空密码"。
func TestUnsealGarbageFails(t *testing.T) {
	if _, err := unsealPassword("win-dpapi:bm90LWEtdmFsaWQtYmxvYg=="); err == nil {
		t.Fatal("损坏的 DPAPI 密文应当解密失败")
	}
	if _, err := unsealPassword("bad-scheme:QUJD"); err == nil {
		t.Fatal("未知 scheme 应当报错")
	}
}

// TestSaveLoadRoundTrip 保存 → 加载 必须完整还原
func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)

	cfg := NewConfig()
	cfg.Username = "20210001"
	cfg.Password = "secret-pw"
	cfg.CourseName = "高等数学"
	cfg.Threads = 8

	if err := saveConfigTo(path, cfg); err != nil {
		t.Fatalf("saveConfigTo 失败: %v", err)
	}

	// 落盘内容不得包含明文密码
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取配置失败: %v", err)
	}
	if strings.Contains(string(raw), "secret-pw") {
		t.Fatal("配置文件中出现明文密码")
	}

	got, migrated, err := loadConfigFrom(path)
	if err != nil {
		t.Fatalf("loadConfigFrom 失败: %v", err)
	}
	if migrated {
		t.Fatal("刚保存的 v2 配置不应触发迁移标记")
	}
	if got.Password != "secret-pw" || got.Username != "20210001" ||
		got.CourseName != "高等数学" || got.Threads != 8 {
		t.Fatalf("round-trip 丢失字段: %+v", got)
	}
}

// TestLegacyV1Migration v1（password_b64 伪加密）加载后自动迁移为 v2
func TestLegacyV1Migration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)

	// 手工构造 v1 格式配置
	v1 := map[string]interface{}{
		"username":     "20210001",
		"password_b64": base64.StdEncoding.EncodeToString([]byte("legacy-pw")),
		"node_url":     "节点1（推荐）",
		"hour":         12,
		"minute":       0,
		"advance":      200,
		"threads":      10,
		"course_type":  "online",
		"min_credit":   0,
	}
	data, _ := json.MarshalIndent(v1, "", "  ")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("写入 v1 配置失败: %v", err)
	}

	cfg, migrated, err := loadConfigFrom(path)
	if err != nil {
		t.Fatalf("loadConfigFrom 失败: %v", err)
	}
	if !migrated {
		t.Fatal("v1 配置应触发迁移标记")
	}
	if cfg.Password != "legacy-pw" {
		t.Fatalf("v1 密码还原失败: got %q", cfg.Password)
	}

	// 模拟 LoadConfig 的迁移回写
	if err := saveConfigTo(path, cfg); err != nil {
		t.Fatalf("迁移回写失败: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "legacy-pw") {
		t.Fatal("迁移后仍含明文密码")
	}
	if strings.Contains(string(raw), "password_b64") {
		t.Fatal("迁移后不应再写出 v1 字段")
	}
	if !strings.Contains(string(raw), "password_v2") {
		t.Fatal("迁移后应包含 password_v2 字段")
	}

	// 迁移后的文件可正常二次加载
	cfg2, migrated2, err := loadConfigFrom(path)
	if err != nil {
		t.Fatalf("二次加载失败: %v", err)
	}
	if migrated2 {
		t.Fatal("已迁移的配置不应再次触发迁移")
	}
	if cfg2.Password != "legacy-pw" {
		t.Fatalf("二次加载密码丢失: got %q", cfg2.Password)
	}
}

// TestUndecryptableV2ResetsPassword 无法解密的 v2 密文（跨机器拷贝场景）：
// 密码置空 + 触发回写清除无效密文，但不得让加载整体失败。
func TestUndecryptableV2ResetsPassword(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)

	// 伪造一段无法解密的 v2 密文
	v2 := map[string]interface{}{
		"username":    "20210001",
		"password_v2": "win-dpapi:AAAA-bad-blob",
		"hour":        12,
		"minute":      0,
		"threads":     10,
	}
	data, _ := json.MarshalIndent(v2, "", "  ")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("写入配置失败: %v", err)
	}

	cfg, migrated, err := loadConfigFrom(path)
	if err != nil {
		t.Fatalf("损坏密文不应导致加载失败: %v", err)
	}
	if cfg.Password != "" {
		t.Fatalf("无法解密的密文应置空密码, got %q", cfg.Password)
	}
	if !migrated {
		t.Fatal("损坏密文应触发回写（清除无效密文）")
	}
}
