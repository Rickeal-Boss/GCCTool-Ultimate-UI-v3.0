package model

import "strings"

// ─────────────────────────────────────────────────────────────────────────────
// 课程类型：单一映射源（Single Source of Truth）
//
// 历史 Bug（V3.0）：UI 单选组直接存中文标签（"普通网课"），
// 而 model.Course.Match() 与 client.getCourseTypeCode() 比较的是英文代码
// （"online"/"pe"/"normal"），导致：
//   - Match() 里三个 if 全部不成立 → 课程类型过滤完全失效（选体育课也会去抢网课）
//   - getCourseTypeCode(中文) 走 default → 恒返回 "10" → 不管选什么都只查询网课
//
// 修复策略：所有跨层传递一律使用 canonical 代码（online/pe/normal），
// 中文标签只在 UI 展示层出现，转换集中在本文件，避免再次出现"展示值当业务值"。
// ─────────────────────────────────────────────────────────────────────────────

// 规范课程类型代码（跨层传递的唯一合法值）
const (
	CourseTypeOnline = "online" // 网课
	CourseTypePE     = "pe"     // 体育课
	CourseTypeNormal = "normal" // 普通课
)

// courseTypeLabelOrder UI 展示顺序
var courseTypeLabelOrder = []string{"普通网课", "体育课", "普通课"}

// courseTypeLabelToCode 中文标签 → 规范代码
var courseTypeLabelToCode = map[string]string{
	"普通网课": CourseTypeOnline,
	"网课":   CourseTypeOnline,
	"体育课":  CourseTypePE,
	"普通课":  CourseTypeNormal,
}

// courseTypeCodeToLabel 规范代码 → 中文标签（用于回填 UI）
var courseTypeCodeToLabel = map[string]string{
	CourseTypeOnline: "普通网课",
	CourseTypePE:     "体育课",
	CourseTypeNormal: "普通课",
}

// courseTypeCodeToKklxdm 规范代码 → 正方查询参数 kklxdm
//
// ⚠️ 注意：kklxdm 是学校自配的课程类别代码，10/20/30 是当前部署的取值。
// 若学校调整了类别代码，只需修改此表（V3.0 中该映射散落在 client 包里，
// 且与 Match() 里的硬编码各写一份，容易出现不一致）。
var courseTypeCodeToKklxdm = map[string]string{
	CourseTypeOnline: "10",
	CourseTypePE:     "20",
	CourseTypeNormal: "30",
}

// CourseTypeLabels 返回 UI 展示用的课程类型标签（副本，防止调用方意外修改）
func CourseTypeLabels() []string {
	return append([]string(nil), courseTypeLabelOrder...)
}

// NormalizeCourseType 把任意输入（中文标签或英文代码）统一为规范代码。
//
// 容错行为：
//   - 空值 / 空白 / 未知值 → 回落到网课（与 V3.0 的默认行为一致，避免 panic 或空查询）
//   - 中文标签 → 对应规范代码
//   - 已是规范代码 → 原样返回
//
// 这样无论 UI 传中文还是代码传英文，Match() 与查询参数都能得到正确结果。
func NormalizeCourseType(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return CourseTypeOnline
	}
	if code, ok := courseTypeLabelToCode[v]; ok {
		return code
	}
	if _, ok := courseTypeCodeToLabel[v]; ok {
		return v
	}
	return CourseTypeOnline
}

// CourseTypeLabel 规范代码 → 中文标签（供 UI 回填；未知值按网课处理）
func CourseTypeLabel(code string) string {
	if label, ok := courseTypeCodeToLabel[NormalizeCourseType(code)]; ok {
		return label
	}
	return courseTypeLabelOrder[0]
}

// CourseTypeCodeKklxdm 规范代码 → 正方 kklxdm 查询参数
func CourseTypeCodeKklxdm(code string) string {
	if v, ok := courseTypeCodeToKklxdm[NormalizeCourseType(code)]; ok {
		return v
	}
	return courseTypeCodeToKklxdm[CourseTypeOnline]
}
