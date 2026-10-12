package model

import (
	"regexp"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
// 选课轮次（页签）解析
//
// 实机抓包结论（2026-09-15 / 2026-10-12，jwxt.gcc.edu.cn）：
//  1. 选课首页的每个页签 = 一整组会话参数：
//     queryCourse(element, kklxdm, xkkz_id, njdm_id, zyh_id, xkkz_xh)
//     站点 JS（zzxkYzb.js）切换页签时把整组值写进隐藏字段，再加载 Display。
//  2. 首页只把"第一个页签"写成 firstKklxdm/firstXkkzId/firstXkkzXh/...，
//     且站点注释明确「只有一个页签时，不显示页签」。
//  3. kklxdm（开放轮次的课程类型代码）每学期都会变：
//     2026-09 第 8 轮 = 10（通识选修课）；2026-10 第 1 轮 = 11（其他特殊课程）。
//     硬编码候选表不可靠，必须以页面实际下发的轮次为准。
//
// 关键教训：轮次参数必须"成组取用"。把 A 类型的 kklxdm 与 B 轮次的 xkkz_xh
// 混着发，服务端只回一句「加密串错误」——错误信息掩盖真实原因，是排查最贵的坑。
// ─────────────────────────────────────────────────────────────────────────────

// Round 一个选课轮次（页签）对应的完整会话参数组
type Round struct {
	Kklxdm string // 课程类型代码（页签标识，如 10/11/20/01）
	Name   string // 页签名（如"通识选修课"）；无页签时为空
	XkkzID string // 选课配置 ID（页面下发）
	XkkzXh string // 加密串（选课提交必需，缺失时服务端回"加密串错误"）
	NjdmID string // 年级代码
	ZyhID  string // 专业代码
}

// IsComplete 参数齐备才算有效轮次。
// 残缺页签（缺 kklxdm/xkkz_id/xkkz_xh 任一）必须直接丢弃：
// 半组参数发出去只会得到误导性的「加密串错误」。
func (r Round) IsComplete() bool {
	return r.Kklxdm != "" && r.XkkzID != "" && r.XkkzXh != ""
}

// queryCourseCallRe 匹配站点 JS 的页签切换调用。
// 形如：queryCourse(this, '10', '1234', '2023', '0801', '6a3f...')
// 第一个参数是 DOM 元素，忽略；其余 5 个按位置取。
var queryCourseCallRe = regexp.MustCompile(
	`queryCourse\s*\(\s*[^,]*?\s*,\s*['"]([^'"]*)['"]\s*,\s*['"]([^'"]*)['"]\s*,\s*['"]([^'"]*)['"]\s*,\s*['"]([^'"]*)['"]\s*,\s*['"]([^'"]*)['"]`)

// tabAnchorRe 匹配页签锚点及其文本，用于取页签名。
// 形如：<li id="tab_kklx_10"><a ...>通识选修课</a></li>
var tabAnchorRe = regexp.MustCompile(`(?is)tab_kklx_([0-9a-z_]+)[^>]*>\s*(?:<[^>]+>\s*)*([^<]+)`)

// inputTagRe 匹配任意 <input ...> 标签（属性顺序不定，逐一解析）
var inputTagRe = regexp.MustCompile(`(?is)<input\b([^>]*)>`)

// htmlAttrRe 匹配单个 HTML 属性
var htmlAttrRe = regexp.MustCompile(`(?i)([a-z_][a-z0-9_\-]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)

// ParseRounds 从选课首页 HTML 解析轮次（页签）列表。
//
// 行为：
//   - 有页签（queryCourse 调用）→ 逐个解析，丢弃残缺页签；
//   - 无页签（只有一个轮次开放，站点不渲染页签）→ 用首页 firstXxx 合成单轮次；
//   - 两者都没有 → 返回 nil（调用方按"无轮次信息"处理）。
//
// 页签名解析失败不影响轮次可用性（Name 仅用于日志展示）。
func ParseRounds(indexHTML string) []Round {
	if strings.TrimSpace(indexHTML) == "" {
		return nil
	}

	// 页签名速查：tab_kklx_<kklxdm> → 文本
	names := make(map[string]string)
	for _, m := range tabAnchorRe.FindAllStringSubmatch(indexHTML, -1) {
		code := strings.TrimSpace(m[1])
		// m[1] 可能是 "10" 也可能是 "10_1" 这类后缀，取首段对齐 kklxdm
		if i := strings.IndexByte(code, '_'); i > 0 {
			code = code[:i]
		}
		text := strings.TrimSpace(m[2])
		if code != "" && text != "" {
			if _, exists := names[code]; !exists {
				names[code] = text
			}
		}
	}

	var rounds []Round
	for _, m := range queryCourseCallRe.FindAllStringSubmatch(indexHTML, -1) {
		r := Round{
			Kklxdm: strings.TrimSpace(m[1]),
			XkkzID: strings.TrimSpace(m[2]),
			NjdmID: strings.TrimSpace(m[3]),
			ZyhID:  strings.TrimSpace(m[4]),
			XkkzXh: strings.TrimSpace(m[5]),
		}
		if !r.IsComplete() {
			continue // 残缺页签直接丢弃
		}
		if name, ok := names[r.Kklxdm]; ok {
			r.Name = name
		}
		rounds = append(rounds, r)
	}
	if len(rounds) > 0 {
		return dedupRounds(rounds)
	}

	// 无页签：用首页 firstXxx 合成单轮次（单轮次开放时的常态）
	hidden := parseHiddenFields(indexHTML)
	first := Round{
		Kklxdm: hidden["firstKklxdm"],
		Name:   hidden["firstKklxmc"],
		XkkzID: hidden["firstXkkzId"],
		NjdmID: hidden["firstNjdmId"],
		ZyhID:  hidden["firstZyhId"],
		XkkzXh: hidden["firstXkkzXh"],
	}
	if first.IsComplete() {
		return []Round{first}
	}
	return nil
}

// dedupRounds 按 kklxdm 去重（保留首个完整页签）
func dedupRounds(in []Round) []Round {
	seen := make(map[string]bool, len(in))
	out := make([]Round, 0, len(in))
	for _, r := range in {
		if seen[r.Kklxdm] {
			continue
		}
		seen[r.Kklxdm] = true
		out = append(out, r)
	}
	return out
}

// parseHiddenFields 提取页面 hidden input 的 name → value
func parseHiddenFields(pageHTML string) map[string]string {
	out := make(map[string]string)
	for _, tag := range inputTagRe.FindAllStringSubmatch(pageHTML, -1) {
		attrs := parseHTMLAttrs(tag[1])
		if !strings.EqualFold(attrs["type"], "hidden") {
			continue
		}
		if name := attrs["name"]; name != "" {
			out[name] = attrs["value"]
		}
	}
	return out
}

// parseHTMLAttrs 解析单个标签内的全部属性（key 统一小写）
func parseHTMLAttrs(tagBody string) map[string]string {
	attrs := make(map[string]string)
	for _, m := range htmlAttrRe.FindAllStringSubmatch(tagBody, -1) {
		key := strings.ToLower(m[1])
		switch {
		case m[2] != "":
			attrs[key] = m[2]
		case m[3] != "":
			attrs[key] = m[3]
		case m[4] != "":
			attrs[key] = m[4]
		default:
			if _, exists := attrs[key]; !exists {
				attrs[key] = ""
			}
		}
	}
	return attrs
}
