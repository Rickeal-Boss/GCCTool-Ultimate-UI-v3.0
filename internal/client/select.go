package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
)

// ErrSelectRetry 服务端返回 flag = -1：并非真正失败，而是"本轮未受理"
// （实机 msg 形如 `0,<jxb_id>,<容量>,` —— 已满/余量0 的数据回显）。
// robber 应据此走"短延迟重试"而不是当成错误刷屏。
var ErrSelectRetry = errors.New("选课待重试（服务端返回 flag=-1）")

// ErrQuotaExceeded 超过本学期选课门次上限。
//
// 实机结论：msg 形如"超过通识选修课本学期最高选课门次限制"。
// 这**不是选课成功**（曾照抄 Efarxs 当成成功处理 → 打印"✅ 选课成功！
// 停止所有 Worker"，实际一门没选中，静默假成功）。重试也无效，必须停止。
var ErrQuotaExceeded = errors.New("选课门次已达上限")

// submitMinInterval 提交类请求（选课/退课）的全局限速间隔。
//
// 背景：10 个 Worker × 每轮 5 次提交 = 提交风暴，而服务端限流阈值很低
// （实测连续 4~7 个请求即 429）。
// ⚠️ 教训：限速必须在**锁内** sleep —— 锁外 sleep 时并发调用会同时醒来，
// 等于没限速（测试实测：并发 4 次仅间隔 301ms 而非期望的 900ms）。
const submitMinInterval = 300 * time.Millisecond

// submitMu / lastSubmitAt 提交全局限速（锁内等待，见 throttleSubmit）
var (
	submitMu     sync.Mutex
	lastSubmitAt time.Time
)

// throttleSubmit 提交前限速（全局串行化到 ≥300ms 间隔）
func (c *Client) throttleSubmit() {
	submitMu.Lock()
	defer submitMu.Unlock()
	if wait := submitMinInterval - time.Since(lastSubmitAt); wait > 0 {
		time.Sleep(wait)
	}
	lastSubmitAt = time.Now()
}

// SelectCourse 选课（正方 V9）
//
// 请求次数：仅发 1 次 POST 到选课提交接口（之前有全局限速，见 throttleSubmit）。
// 初始化参数（xkkz_id 等）由 GetClassList 调用时已解析并缓存在参数池中，
// 此处**按端点白名单**取用回填 —— 整包回填会被服务端以「系统运行异常」拒绝
// （2026-09-15 实机对照实验：白名单 flag=1 / 整包参数池被拒）。
//
// 正方 V9 关键点：
//   - 必须使用 do_jxb_id（加密长 ID），不能用短 jxb_id
//   - kcmc 格式为 "(课程号)课程名-学分学分"（浏览器实际提交格式）
//   - sxbj 按 rlkz 计算，qz 固定 0
//   - 所有接口 URL 需带 ?gnmkdm=N253512
func (c *Client) SelectCourse(course *model.Course) error {
	params := c.buildSelectParams(course)

	c.throttleSubmit()

	selectURL := c.buildURL(pathSelectSubmit) + "?gnmkdm=" + gnmkdmSelect
	resp, err := c.doPost(selectURL, params)
	if err != nil {
		return err
	}

	if err := c.parseSelectResult(resp); err != nil {
		return err
	}

	// 选课成功后作废课程列表缓存，避免后续轮询继续使用"选课之前"的旧课表
	c.InvalidateCourseListCache()
	return nil
}

// buildSelectParams 构建选课提交参数
//
// 修复点（V3.0 bug）：原实现只提交 kch_id / kcmc / kklxdm / gnmkdm / jxb_ids 五个字段，
// 而正方 V9 的选课提交接口还需要选课首页/Display 页下发的开关参数
// （xkkz_id、rwlx、rlkz、sxbj、cxbj、qz、xklc、xkxnm、xkxqm、njdm_id、zyh_id 等）。
// 缺参数的服务端表现通常是直接拒绝，且返回文案含糊，极难排查。
//
// 修复策略（2026-09-15 实机定稿）：**从参数池按 selectParamNames 白名单取字段**，
// 再用轮次整组参数与本次课程特有的字段覆盖。白名单之外的字段一律不提交 ——
// 整包发送会被服务端拒绝（表单只提交自己 form 的字段）。
func (c *Client) buildSelectParams(course *model.Course) map[string]string {
	params := pickWhitelisted(c.SelectInitParams(), selectParamSet)

	// 轮次整组参数（与列表/详情同源；混用不同轮次的参数只会得到误导性的
	// 「加密串错误」，掩盖真实原因）
	round, _ := c.roundFor(course.Type)
	// kklxdm 优先用课程自身类型（来自列表响应，是该课程的 ground truth）；
	// 课程缺类型时才回落轮次/参数池的值
	if course.Type != "" {
		params["kklxdm"] = course.Type
	} else if round.Kklxdm != "" {
		params["kklxdm"] = round.Kklxdm
	}
	if round.XkkzID != "" {
		params["xkkz_id"] = round.XkkzID
	}
	if round.XkkzXh != "" {
		params["xkkz_xh"] = round.XkkzXh
	}
	if round.NjdmID != "" {
		params["njdm_id"] = round.NjdmID
	}
	if round.ZyhID != "" {
		params["zyh_id"] = round.ZyhID
	}

	// ── 本次选课特有的字段（覆盖页面下发的同名值）───────────────────────────
	params["kch_id"] = course.ID
	params["kcmc"] = formatKcmc(course)

	// do_jxb_id：V9 使用加密长 ID；Extra 未取到时降级使用短 jxb_id
	if course.Extra != nil && course.Extra.DoJxbID != "" {
		params["jxb_ids"] = course.Extra.DoJxbID
	} else {
		params["jxb_ids"] = course.ClassID
	}

	// sxbj 按 rlkz 计算（实机抓包口径）；qz 固定 0
	params["sxbj"] = sxbjFromRlkz(params["rlkz"])
	params["qz"] = "0"

	return params
}

// formatKcmc 组装选课提交的课程名字段：(课程号)课程名-学分学分
// 例：(课程号)课程名-2.0学分
func formatKcmc(course *model.Course) string {
	credit := course.CreditRaw
	if credit == "" {
		credit = strconv.Itoa(course.Credit)
	}
	return fmt.Sprintf("(%s)%s-%s学分", course.Number, course.Name, credit)
}

// sxbjFromRlkz 按 rlkz（选课开关）计算 sxbj（实机抓包口径：rlkz=1 → 1，否则 0）
func sxbjFromRlkz(rlkz string) string {
	if strings.TrimSpace(rlkz) == "1" {
		return "1"
	}
	return "0"
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
//  2. 没有实现 flag = -1（本轮未受理）分支。
//
// 现在用 map + 容错取值：flag 是数字或字符串都能识别，并区分
// "成功 / 待重试 / 门次上限 / 真失败" 四种语义。
//
// 实机结论（2026-09-15）：
//   - flag=1：成功；
//   - flag=-1：本轮未受理（已满），msg 形如 `0,<jxb_id>,<容量>,`；
//   - msg 含"门次"：超过选课门次上限（ErrQuotaExceeded，**不是成功**，重试无效）。
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

	// 门次上限：超过本学期最高选课门次限制 —— 不是成功，重试无效
	if strings.Contains(message, "门次") {
		return fmt.Errorf("%w: %s（重试无效，请先退掉已选课程或联系教务处）", ErrQuotaExceeded, message)
	}

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
		// 实机 msg 形如 "0,5A901A89CE325A77E065000000000001,90,"（余量0/已满回显）
		return fmt.Errorf("%w: %s", ErrSelectRetry, msgOr(message, "本轮未选上(flag=-1，通常为已满/未受理)"))
	default:
		return fmt.Errorf("选课失败: %s", msgOr(message, "服务端未返回失败原因"))
	}
}

// QuerySelectedCourse 查询已选课程
//
// 实机结论（2026-09-15）：
//  1. 端点必须是 cxZzxkYzbChoosedDisplay.html（原 cxYxkAndKc.html 被 HTTP 910 拦截；
//     cxZzxkYzbChoosed.html 不是浏览器实际使用的接口）；
//  2. 参数 {flag:1} 恒返回 []（静默假象）；整包参数池 → 系统异常；
//     必须按白名单取参；
//  3. 响应是**顶层 JSON 数组**（ParseCourseList 已兼容）。
func (c *Client) QuerySelectedCourse() (*model.CourseList, error) {
	pool := c.SelectInitParams()
	if len(pool) == 0 {
		return nil, fmt.Errorf("尚未获取选课参数（请先查询一次课程列表建立参数池）")
	}

	queryURL := c.buildURL(pathSelectedCourses) + "?gnmkdm=" + gnmkdmSelect
	params := pickWhitelisted(pool, browserListParamSet)

	resp, err := c.doPost(queryURL, params)
	if err != nil {
		return nil, err
	}

	return model.ParseCourseList([]byte(resp))
}

// CancelCourse 退课
//
// 实机结论（2026-09-15，站点 JS zzxkYzbChoosedZy.js 的 delCourse()）：
//   - 端点 = /xsxk/zzxkyzb_tuikBcZzxkYzb.html（"tuik" 不是 "tk"！
//     原 tkZzxkYzb 服务端明确回「请求的方法 tkZzxkYzb 在对象 ZzxkYzbAction
//     中未定义！」+ HTTP 910）；
//   - 参数只发 5 个字段：{kch_id, jxb_ids(=do_jxb_id), xkxnm, xkxqm, txbsfrl}；
//   - 响应是**裸 JSON 字符串**："1"=成功 2=繁忙 3=未知异常 4=非法访问 5=校验不通过。
func (c *Client) CancelCourse(course *model.Course) error {
	pool := c.SelectInitParams()
	params := pickWhitelisted(pool, cancelParamSet)

	// kch_id / jxb_ids：退课用 do_jxb_id（加密长 ID），拿不到才降级短 jxb_id
	params["kch_id"] = course.ID
	if course.Extra != nil && course.Extra.DoJxbID != "" {
		params["jxb_ids"] = course.Extra.DoJxbID
	} else {
		params["jxb_ids"] = course.ClassID
	}

	c.throttleSubmit()

	cancelURL := c.buildURL(pathCancelCourse) + "?gnmkdm=" + gnmkdmSelect
	resp, err := c.doPost(cancelURL, params)
	if err != nil {
		return err
	}

	if err := c.parseCancelResult(resp); err != nil {
		return err
	}
	// 退课同样改变了可选课表，作废缓存
	c.InvalidateCourseListCache()
	return nil
}

// parseCancelResult 解析退课结果
//
// 退课响应是裸 JSON 字符串（不是选课那种对象）：
//
//	"1"=成功 2=繁忙 3=未知异常 4=非法访问 5=校验不通过
//
// 用选课解析器（parseSelectResult）解析裸字符串必然失败，必须分开处理。
func (c *Client) parseCancelResult(resp string) error {
	trimmed := strings.TrimSpace(resp)
	if trimmed == "" {
		return fmt.Errorf("退课响应为空（可能被风控拦截或会话已失效）")
	}

	// 对象/数组形态：按选课结果解析器兼容处理
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return c.parseSelectResult(resp)
	}

	// 裸字符串形态
	switch trimmed {
	case `"1"`, "1":
		return nil
	case `"2"`, "2":
		return fmt.Errorf("退课失败：服务端繁忙(2)，请稍后重试")
	case `"3"`, "3":
		return fmt.Errorf("退课失败：服务端未知异常(3)")
	case `"4"`, "4":
		return fmt.Errorf("退课失败：非法访问(4)，参数可能不完整")
	case `"5"`, "5":
		return fmt.Errorf("退课失败：校验不通过(5)")
	}
	return fmt.Errorf("退课返回了无法识别的响应（前120字符: %s）", previewOf(trimmed, 120))
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
