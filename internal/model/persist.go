// Package model - 配置持久化
//
// 将用户配置保存到本地 JSON 文件，程序重启后自动加载，
// 避免每次手动重新输入账号、节点、时间等参数。
//
// 安全设计（V3.3 安全加固）：
//   - 密码加密存储：Windows 上使用 DPAPI（CurrentUser 范围）加密，
//     配置文件被拷贝到其他机器/其他用户后无法离线还原密码；
//     非 Windows 平台回落 Base64（仅混淆，见 persist_secure_other.go）。
//   - 防护边界（如实声明）：DPAPI 防「文件离开本机 / 其他系统用户读取」，
//     不防「同一用户下运行的恶意软件」——恶意软件可以调用同样的 API 解密。
//     对抗后者需要每次启动手动输密码（不落盘），属可用性取舍，默认不做。
//   - 旧版本配置（password_b64，Base64 伪加密）在加载时自动迁移为 v2 格式。
//   - 文件权限 0600 仅在类 Unix 平台生效；Windows 通过 NTFS 用户目录
//     隔离 + DPAPI 提供保护（0600 权限位在 Windows 上会被忽略）。
package model

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const configFileName = "gcctool_config.json"

// configPersist 持久化格式（与 Config 分离，避免直接存储敏感字段）
type configPersist struct {
	// 账号（学号为低敏感 PII，保留明文便于用户自查配置；
	// 高敏感的密码走 PasswordV2 加密通道）
	Username string `json:"username"`

	// PasswordV2 加密后的密码（v2 格式）："scheme:base64密文"。
	// scheme=win-dpapi 表示 Windows DPAPI（CurrentUser）加密。
	PasswordV2 string `json:"password_v2,omitempty"`

	// PasswordB64 v1 兼容字段（Base64 伪加密，历史遗留）。
	// 仅在读取时兼容，加载后自动迁移到 PasswordV2，不再写出。
	PasswordB64 string `json:"password_b64,omitempty"`

	// 节点配置
	NodeURL string `json:"node_url"`
	Agent   string `json:"agent,omitempty"`

	// 选课时间
	Hour    int `json:"hour"`
	Minute  int `json:"minute"`
	Advance int `json:"advance"`

	// 并发配置
	Threads int `json:"threads"`

	// 课程筛选
	CourseType   string `json:"course_type"`
	CourseName   string `json:"course_name,omitempty"`
	TeacherName  string `json:"teacher_name,omitempty"`
	CourseNumber string `json:"course_number,omitempty"`
	MinCredit    int    `json:"min_credit"`

	// 课程分类
	Categories map[string]bool `json:"categories,omitempty"`
}

// SaveConfig 将配置保存到本地文件
//
// 存储位置：程序运行目录下的 gcctool_config.json
// 密码加密失败时拒绝写入（保持旧文件原状），而不是降级为明文落盘。
func SaveConfig(cfg *Config) error {
	return saveConfigTo(configPath(), cfg)
}

// saveConfigTo 写入指定路径（测试通过 t.TempDir() 注入隔离目录）
func saveConfigTo(path string, cfg *Config) error {
	sealed, err := sealPassword(cfg.Password)
	if err != nil {
		return fmt.Errorf("密码加密失败，已放弃写入配置（原文件保持不变）: %w", err)
	}

	p := configPersist{
		Username:    cfg.Username,
		PasswordV2:  sealed,
		NodeURL:     cfg.NodeURL,
		Agent:       cfg.Agent,
		Hour:        cfg.Hour,
		Minute:      cfg.Minute,
		Advance:     cfg.Advance,
		Threads:     cfg.Threads,
		CourseType:  cfg.CourseType,
		CourseName:  cfg.CourseName,
		TeacherName: cfg.TeacherName,
		CourseNumber: cfg.CourseNumber,
		MinCredit:   cfg.MinCredit,
		Categories:  cfg.Categories,
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}

	// 权限 0600：仅类 Unix 平台生效；Windows 忽略该位（见文件头说明）
	return os.WriteFile(path, data, 0600)
}

// LoadConfig 从本地文件加载配置
//
// 若文件不存在则返回默认配置，不报错。
// 兼容 v1（password_b64）：加载成功后自动回写迁移为 v2。
// v2 密文解密失败（典型场景：配置文件从别的机器拷来，DPAPI 密文
// 与当前用户/机器不匹配）时置空密码让用户重输，不阻塞启动。
func LoadConfig() (*Config, error) {
	cfg, migrated, err := loadConfigFrom(configPath())
	if err != nil {
		return cfg, err
	}
	if migrated {
		// 尽力迁移（v1 → v2，或清除无法解密的旧密文）；失败不影响启动
		_ = saveConfigTo(configPath(), cfg)
	}
	return cfg, nil
}

// loadConfigFrom 从指定路径加载（测试注入隔离目录）。
// 第二个返回值表示是否需要回写迁移。
func loadConfigFrom(path string) (*Config, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// 首次运行，返回默认配置
		return NewConfig(), false, nil
	}
	if err != nil {
		return NewConfig(), false, err
	}

	var p configPersist
	if err := json.Unmarshal(data, &p); err != nil {
		// 配置文件损坏，返回默认配置
		return NewConfig(), false, nil
	}

	cfg := NewConfig()
	cfg.Username = p.Username
	cfg.NodeURL = p.NodeURL
	cfg.Agent = p.Agent
	cfg.Hour = p.Hour
	cfg.Minute = p.Minute
	cfg.Advance = p.Advance
	// 修复（V3.0 bug）：旧配置文件里 Threads 可能缺失或被写成 0，
	// 直接赋值会让"并发线程数=0"静默生效（startWorkers 的 wg.Add(0) 一个请求都不发）。
	// 这里对 0/负数回落到默认值。
	if p.Threads > 0 {
		cfg.Threads = p.Threads
	}
	cfg.CourseType = p.CourseType
	cfg.CourseName = p.CourseName
	cfg.TeacherName = p.TeacherName
	cfg.CourseNumber = p.CourseNumber
	cfg.MinCredit = p.MinCredit
	if p.Categories != nil {
		cfg.Categories = p.Categories
	}

	// 归一化可容错字段：
	//   - 旧版本把中文标签直接存进 course_type，归一化后即可被 Match()/查询正确识别
	//   - NodeURL 为空（旧配置）时回落到默认节点
	cfg.Normalize()
	if cfg.NodeURL == "" {
		cfg.NodeURL = NewConfig().NodeURL
	}

	// ── 密码还原（v2 优先，v1 兼容迁移）──────────────────────────────────────
	migrated := false
	switch {
	case p.PasswordV2 != "":
		plain, err := unsealPassword(p.PasswordV2)
		if err != nil {
			// 跨机器拷贝的 DPAPI 密文（或损坏数据）：置空让用户重输，
			// 同时清掉无效密文（migrated 触发回写）。
			cfg.Password = ""
			migrated = true
		} else {
			cfg.Password = plain
		}
	case p.PasswordB64 != "":
		// v1（Base64 伪加密）→ 解出并标记迁移，加载完成后回写为 v2
		if decoded, err := base64.StdEncoding.DecodeString(p.PasswordB64); err == nil {
			cfg.Password = string(decoded)
		}
		migrated = true
	}

	return cfg, migrated, nil
}

// configPath 返回配置文件的绝对路径（与可执行文件同目录）
func configPath() string {
	exe, err := os.Executable()
	if err != nil {
		return configFileName
	}
	return filepath.Join(filepath.Dir(exe), configFileName)
}

// ConfigExists 检查本地是否已有保存的配置
func ConfigExists() bool {
	_, err := os.Stat(configPath())
	return err == nil
}

// ─────────────────────────────────────────────────────────────────────────────
// 密码落盘加密（平台相关实现见 persist_secure_windows.go / persist_secure_other.go）
// ─────────────────────────────────────────────────────────────────────────────

const (
	// passwordSchemeWindows Windows DPAPI（CurrentUser）加密
	passwordSchemeWindows = "win-dpapi"
	// passwordSchemePlain64 Base64 混淆（非 Windows 平台回落方案，非加密）
	passwordSchemePlain64 = "plain64"
)

// splitSealed 拆出 "scheme:base64" 的两段
func splitSealed(sealed string) (scheme, b64 string, ok bool) {
	scheme, b64, ok = strings.Cut(sealed, ":")
	if scheme == "" || b64 == "" {
		return "", "", false
	}
	return scheme, b64, true
}
