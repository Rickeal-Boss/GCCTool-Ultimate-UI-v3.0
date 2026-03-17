// Package model - 配置持久化
//
// 将用户配置保存到本地 JSON 文件，程序重启后自动加载，
// 避免每次手动重新输入账号、节点、时间等参数。
//
// 安全设计：
//   - 密码不明文存储，使用 Base64 轻量混淆（非加密，仅防截图泄露）
//   - 文件权限设置为 0600（仅当前用户可读）
package model

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
)

const configFileName = "gcctool_config.json"

// configPersist 持久化格式（与 Config 分离，避免直接存储敏感字段）
type configPersist struct {
	// 账号（学号明文存储，密码混淆存储）
	Username      string `json:"username"`
	PasswordB64   string `json:"password_b64"` // base64 混淆，防止截图直接看到密码

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
// 文件权限：0600（仅当前用户可读写）
func SaveConfig(cfg *Config) error {
	p := configPersist{
		Username:     cfg.Username,
		PasswordB64:  base64.StdEncoding.EncodeToString([]byte(cfg.Password)),
		NodeURL:      cfg.NodeURL,
		Agent:        cfg.Agent,
		Hour:         cfg.Hour,
		Minute:       cfg.Minute,
		Advance:      cfg.Advance,
		Threads:      cfg.Threads,
		CourseType:   cfg.CourseType,
		CourseName:   cfg.CourseName,
		TeacherName:  cfg.TeacherName,
		CourseNumber: cfg.CourseNumber,
		MinCredit:    cfg.MinCredit,
		Categories:   cfg.Categories,
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}

	// 权限 0600：仅当前用户可读写
	return os.WriteFile(configPath(), data, 0600)
}

// LoadConfig 从本地文件加载配置
//
// 若文件不存在则返回默认配置，不报错。
func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		// 首次运行，返回默认配置
		return NewConfig(), nil
	}
	if err != nil {
		return NewConfig(), err
	}

	var p configPersist
	if err := json.Unmarshal(data, &p); err != nil {
		// 配置文件损坏，返回默认配置
		return NewConfig(), nil
	}

	cfg := NewConfig()
	cfg.Username = p.Username
	cfg.NodeURL = p.NodeURL
	cfg.Agent = p.Agent
	cfg.Hour = p.Hour
	cfg.Minute = p.Minute
	cfg.Advance = p.Advance
	cfg.Threads = p.Threads
	cfg.CourseType = p.CourseType
	cfg.CourseName = p.CourseName
	cfg.TeacherName = p.TeacherName
	cfg.CourseNumber = p.CourseNumber
	cfg.MinCredit = p.MinCredit
	if p.Categories != nil {
		cfg.Categories = p.Categories
	}

	// 还原密码
	if p.PasswordB64 != "" {
		if decoded, err := base64.StdEncoding.DecodeString(p.PasswordB64); err == nil {
			cfg.Password = string(decoded)
		}
	}

	return cfg, nil
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
