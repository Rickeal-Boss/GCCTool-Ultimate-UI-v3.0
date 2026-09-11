package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
)

// ErrSelectRetry 服务端返回 flag = -1：并非真正失败，而是要求稍后重试
// （排队、瞬时拒绝等）。robber 应据此走"短延迟重试"而不是当成错误刷屏。
var ErrSelectRetry = errors.New("选课待重试（服务端返回 flag=-1）")

// SelectCourse 选课（正方 V9）
//
// 请求次数：仅发 1 次 POST 到选课提交接口。
// 初始化参数（xkkz_id 等）由 GetClassList 调用时已从选课首页提取并缓存在 Client 中，
// 此处直接回填，避免每轮选课重复 GET 首页 + POST display。
//
// 正方 V9 关键点：
//   - 必须使用 do_jxb_id（加密长 ID），不能用短 jxb_id
//   - 所有接口 URL 需带 ?gnmkdm=N253512
func (c *Client) SelectCourse(course *model.Course) error {
	params := c.buildSelectParams(course)

	selectURL := c.buildURL(pathSelectSubmit) + "?gnmkdm=" + gnmkdmSelect
	resp, err := c.doPost(selectURL, params)
	if err != nil {
		return err
	}

	return c.parseSelectResult(resp)
}

// buildSelectParams 构建选课提交参数
//
// 修复点（V3.0 bug）：原实现只提交 kch_id / kcmc / kklxdm / gnmkdm / jxb_ids 五个字段，
// 而正方 V9 的选课提交接口还需要选课首页/Display 页下发的开关参数
// （xkkz_id、rwlx、rlkz、sxbj、cxbj、qz、xklc、xkxnm、xkxqm、njdm_id、zyh_id 等）。
// 缺参数的服务端表现通常是直接拒绝，且返回文案含糊，极难排查。
//
// 修复策略：从缓存的页面初始化参数出发，整体回填，再用本次课程特有的字段覆盖。
// 这样不写死字段名单 —— 学校换正方版本时页面下发什么就提交什么。
func (c *Client) buildSelectParams(course *model.Course) map[string]string {
	params := c.SelectInitParams()
	if params == nil {
		params = make(map[string]string, 8)
	}

	// ── 本次选课特有的字段（覆盖页面下发的同名值）───────────────────────────
	params["kch_id"] = course.ID
	params["kcmc"] = course.Name
	params["kklxdm"] = course.Type
	params["gnmkdm"] = gnmkdmSelect

	// do_jxb_id：V9 使用加密长 ID；Extra 未取到时降级使用短 jxb_id
	if course.Extra != nil && course.Extra.DoJxbID != "" {
		params["jxb_ids"] = course.Extra.DoJxbID
	} else {
		params["jxb_ids"] = course.ClassID
	}

	return params
}

// SelectInitParamKeys 返回当前缓存的选课初始化参数名（已排序，用于日志排查）
//
// 用途：选课失败时把实际提交的参数名列出来，便于对照浏览器抓包确认
// 是否缺少关键开关参数（如 xkkz_id）。
func (c *Client) SelectInitParamKeys() []string {
	keys := make([]string, 0, 16)
	for k := range c.SelectInitParams() {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// parseSelectResult 解析选课结果
//
// 修复点（V3.0 bug）：
//  1. 原实现用 `Flag string` 接收，而正方返回的 flag 是 JSON number（1 / -1）。
//     数字塞进 string 字段会导致 Unmarshal 直接报错 → 返回"解析选课结果失败"
//     → 即使真的选上了，robber 也认为失败并继续重复提交。
//  2. 没有实现 flag = -1（要求重试）分支。
//
// 现在用 map + 容错取值：flag 是数字或字符串都能识别，并区分
// "成功 / 待重试 / 真失败" 三种语义。
func (c *Client) parseSelectResult(resp string) error {
	trimmed := strings.TrimSpace(resp)
	if trimmed == "" {
		return fmt.Errorf("选课响应为空（可能被风控拦截或会话已失效）")
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		if strings.HasPrefix(trimmed, "<") {
			return fmt.Errorf("选课返回了HTML而非JSON（可能会话已过期或服务端异常）: %s", previewOf(trimmed, 120))
		}
		return fmt.Errorf("解析选课结果失败: %w（响应前120字符: %s）", err, previewOf(trimmed, 120))
	}

	flag, flagOK := model.ToInt(raw["flag"])
	message := firstString(raw, "message", "msg", "message1", "msgText")

	// 兜底：flag 语义异常但响应文案明确表示成功时，按成功处理，避免"选上了却报失败"
	if flag != 1 && containsAny(trimmed, "选课成功", "已成功添加", "添加成功") {
		return nil
	}

	if !flagOK {
		return fmt.Errorf("选课结果缺少可识别的 flag 字段（响应前120字符: %s）", previewOf(trimmed, 120))
	}

	switch flag {
	case 1:
		return nil
	case -1:
		return fmt.Errorf("%w: %s", ErrSelectRetry, msgOr(message, "服务端要求稍后重试"))
	default:
		return fmt.Errorf("选课失败: %s", msgOr(message, "服务端未返回失败原因"))
	}
}

// QuerySelectedCourse 查询已选课程
func (c *Client) QuerySelectedCourse() (*model.CourseList, error) {
	queryURL := c.buildURL(pathSelectedCourses) + "?gnmkdm=" + gnmkdmSelect

	params := map[string]string{
		"flag":   "1",
		"gnmkdm": gnmkdmSelect,
	}

	resp, err := c.doPost(queryURL, params)
	if err != nil {
		return nil, err
	}

	return model.ParseCourseList([]byte(resp))
}

// CancelCourse 退课
func (c *Client) CancelCourse(course *model.Course) error {
	cancelURL := c.buildURL(pathCancelCourse) + "?gnmkdm=" + gnmkdmSelect

	params := map[string]string{
		"kch_id":  course.ID,
		"jxb_ids": course.ClassID,
		"gnmkdm":  gnmkdmSelect,
	}

	resp, err := c.doPost(cancelURL, params)
	if err != nil {
		return err
	}

	return c.parseSelectResult(resp)
}

// ─────────────────────────────────────────────────────────────────────────────
// 辅助函数
// ─────────────────────────────────────────────────────────────────────────────

// firstString 从 map 中按顺序取第一个非空字符串值
func firstString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := model.ToString(v); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// containsAny 判断 s 是否包含任一关键词
func containsAny(s string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// msgOr 返回 message；为空时使用兜底文案
func msgOr(message, fallback string) string {
	if strings.TrimSpace(message) == "" {
		return fallback
	}
	return message
}
