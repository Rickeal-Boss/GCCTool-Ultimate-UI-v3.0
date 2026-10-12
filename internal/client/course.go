package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"golang.org/x/net/html"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/stealth"
)

// GetClassList 获取课程列表
//
// 链路（2026-09-15 实机抓包重写）：
//
//	参数池复用窗口内（90s）：仅 1 个 PartDisplay POST（生产中 4 请求/90s → 1）；
//	窗口外（完整链路）：
//	  1. 不跟随重定向的探测 GET —— 区分 "Session失效(302)" 与 "选课未开放(200页面)"；
//	  2. GET 选课首页 —— 解析隐藏参数 + 轮次（页签）列表；
//	  3. POST Display（浏览器同款 7 参数）—— 其响应下发的隐藏参数入池；
//	     ⚠️ sfkxk=0（本轮不可选课）时 Display 返回 HTTP 911 + 空 jqGrid 信封，
//	     这是**正常业务分支**，容忍后仅用首页参数继续（实测列表 HTTP 200 + 370KB）；
//	  4. POST PartDisplay —— 按端点白名单取参 + xkkz_xh（实机证明必需）；
//	  5. 解析课程列表（0 门课属于正常业务结果，不返回 error）。
//
// 轮次（页签）机制：一个页签 = 一整组会话参数（kklxdm/xkkz_id/xkkz_xh/njdm_id/
// zyh_id）。首页只把第一个页签写成 firstXxx，且"只有一个页签时不显示页签"。
// 列表查询按 cfg.CourseType 的候选 kklxdm 找轮次，**整组取用**——混用不同轮次的
// 参数只会得到误导性的「加密串错误」（掩盖真实原因）。所选类型无开放轮次时
// 回落首个开放轮次，并置 TypeRelaxed + Warning（robber 据此放宽本地类型过滤）。
func (c *Client) GetClassList(cfg *model.Config) (*model.CourseList, error) {
	// 短 TTL 缓存：同一查询条件下的并发轮询复用同一次查询结果，把请求量降低约三个数量级
	// （详见 client.go 中 listCache 字段的说明）。
	cacheKey := c.courseListCacheKey(cfg)
	if cached := c.CachedCourseList(cacheKey); cached != nil {
		return cached, nil
	}

	indexURL := c.buildURL(pathSelectIndex) + "?gnmkdm=" + gnmkdmSelect + "&layout=default"

	var (
		poolParams map[string]string
		rounds     []model.Round
		warning    string
	)

	if params, rnds, ok := c.poolSnapshot(); ok {
		// ── 参数池复用窗口内：跳过探测/首页/Display，直接查列表 ──────────────
		poolParams, rounds = params, rnds
	} else {
		// ── 完整链路 ──────────────────────────────────────────────────────
		// 步骤0: 用不跟随重定向的探测请求判断 Session 状态
		// 原因：http.Client 默认跟随302，会自动跳到登录页，导致拿到登录页HTML后被误判为Session失效
		probeReq, err := http.NewRequest(http.MethodGet, indexURL, nil)
		if err != nil {
			return nil, fmt.Errorf("构建选课页探测请求失败: %w", err)
		}
		stealth.InjectHeaders(probeReq)
		probeReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

		// 使用不跟随重定向的客户端
		probeClient := &http.Client{
			Timeout:   c.httpClient.Timeout,
			Jar:       c.httpClient.Jar,
			Transport: c.httpClient.Transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // 不跟随重定向，直接返回原始302
			},
		}

		probeResp, probeErr := probeClient.Do(probeReq)
		if probeErr == nil {
			defer probeResp.Body.Close()
			// 302/301 跳转到登录页 → Session 真的失效
			if probeResp.StatusCode == http.StatusFound || probeResp.StatusCode == http.StatusMovedPermanently {
				location := probeResp.Header.Get("Location")
				if strings.Contains(location, "login_slogin") || strings.Contains(location, "slogin") {
					return nil, fmt.Errorf("[风控-会话] Session已失效，服务器重定向到登录页(302)")
				}
			}
		}

		// 步骤1: 正常 GET 选课首页（会跟随重定向），获取 hidden input
		indexHTML, err := c.doGet(indexURL)
		if err != nil {
			return nil, err
		}

		// 检测选课未开放 —— 这是正常业务状态，不是Session失效
		if strings.Contains(indexHTML, "不属于选课阶段") ||
			strings.Contains(indexHTML, "不在选课时间") ||
			strings.Contains(indexHTML, "当前不属于选课") {
			return nil, fmt.Errorf("选课未开放：当前不在选课阶段，请等待系统开放后再试")
		}

		// 检测系统维护 —— 同样不是Session失效
		if strings.Contains(indexHTML, "系统正在维护") ||
			strings.Contains(indexHTML, "系统维护") {
			return nil, fmt.Errorf("系统维护：教务系统正在维护，请稍后再试")
		}

		indexHidden := c.parseHiddenInputs(indexHTML)
		rounds = model.ParseRounds(indexHTML)
		round, _ := selectRound(rounds, cfg.CourseType)

		// 步骤2: POST Display（浏览器同款 7 参数）——其响应下发的隐藏参数入池。
		// queryCourse 切页签时会把整组值写进隐藏字段再加载 Display，
		// 因此 Display 必须带**所选轮次**的 xkkz_id/njdm_id/zyh_id（页面里的
		// 初值只是第一个页签的，直接用来请求其他轮次会张冠李戴）。
		displayParams := pickWhitelisted(indexHidden, displayParamSet)
		displayParams["kspage"] = "0"
		displayParams["jspage"] = "0"
		if round.Kklxdm != "" {
			displayParams["kklxdm"] = round.Kklxdm
		}
		if round.XkkzID != "" {
			displayParams["xkkz_id"] = round.XkkzID
		}
		if round.NjdmID != "" {
			displayParams["njdm_id"] = round.NjdmID
		}
		if round.ZyhID != "" {
			displayParams["zyh_id"] = round.ZyhID
		}
		displayURL := c.buildURL(pathSelectDisplay) + "?gnmkdm=" + gnmkdmSelect
		displayHTML, displayErr := c.doPost(displayURL, displayParams)
		if displayErr != nil {
			// sfkxk=0（本轮不可选课）时 Display 返回 HTTP 911 + 空 jqGrid 信封
			//（totalResult=0，无任何 hidden inputs）——正常业务分支，不是故障。
			// 实测此时仅用首页参数就能查课程列表（HTTP 200 + 370KB 真实课程）；
			// Display 提供的 rwlx/xklc/rlkz 只有选课提交才需要。
			if !strings.Contains(displayErr.Error(), "HTTP 911") {
				return nil, displayErr
			}
			warning = "选课开关页(Display)返回 HTTP 911（本轮不可选课 sfkxk=0），已仅用首页参数查询课程列表；此状态下选课提交会被服务端拒绝"
			displayHTML = ""
		}

		// 步骤3: 合并"选课首页 + Display 页"下发的隐藏参数，写入参数池（90s 复用）
		poolParams = make(map[string]string, len(indexHidden)+16)
		for k, v := range indexHidden {
			poolParams[k] = v
		}
		for k, v := range c.parseHiddenInputs(displayHTML) {
			poolParams[k] = v
		}
		c.refreshPool(poolParams, rounds)

		// 页面下发的"课程归属/课程类别"选项：用于把 UI 勾选的中文分类
		// 映射成教务系统真实接受的 kcgs_list 取值（不硬编码任何类别代码）。
		c.SetCategoryOptions(parseCategoryOptions(indexHTML, displayHTML))
	}

	// 步骤4: 按端点白名单构建课程查询参数（整包参数池会被服务端拒绝）
	round, relaxed := selectRound(rounds, cfg.CourseType)
	if relaxed {
		hint := "所选课程类型当前没有开放轮次，已回落到教务实际开放的轮次查询（课程类型过滤已同步放宽）"
		if desc := describeRoundList(rounds); desc != "" {
			hint += "；实际开放轮次：" + desc
		}
		if warning != "" {
			warning += "；" + hint
		} else {
			warning = hint
		}
	}

	postData2 := c.buildCourseQueryParams(poolParams, round, cfg)

	// 步骤5: 获取课程列表JSON
	partURL := c.buildURL(pathCourseList) + "?gnmkdm=" + gnmkdmSelect
	resp, err := c.doPost(partURL, postData2)
	if err != nil {
		return nil, err
	}

	// 服务端业务级拒绝（如 {"flag":"0","msg":"加密串错误"}）不能当成"0 门课"——
	// 那会把协议错误伪装成正常业务结果，用户看到"没排课"以为是自己配置问题。
	if msg, ok := parseErrorEnvelope(resp); ok {
		return nil, fmt.Errorf("课程列表查询被服务端拒绝: %s（如为本学期首轮选课，可能是页面参数尚未刷新，请重试）", msg)
	}

	// 步骤6: 解析课程列表（0 门课属于正常业务结果，不返回 error）
	list, err := model.ParseCourseList([]byte(resp))
	if err != nil {
		// 只缓存成功结果；错误不缓存，避免把一次瞬时的会话/网络故障固化一秒
		return nil, err
	}
	list.Warning = warning
	list.TypeRelaxed = relaxed
	c.CacheCourseList(cacheKey, list)
	// 返回副本：各 Worker 会写入 course.Extra，不能共享同一批指针
	return list.Clone(), nil
}

// selectRound 从轮次列表中为指定课程类型选出轮次（整组取用）。
// 语义同 Client.roundFor，但不读客户端状态（完整链路上轮次刚解析出来、
// 尚未入池，必须用新鲜数据）。
func selectRound(rounds []model.Round, courseType string) (model.Round, bool) {
	candidates := model.CourseTypeKklxdmCandidates(courseType)
	for _, cand := range candidates {
		for _, r := range rounds {
			if r.Kklxdm == cand {
				return r, false
			}
		}
	}
	if len(rounds) > 0 {
		return rounds[0], true
	}
	return model.Round{}, true
}

// describeRoundList 轮次列表的人类可读描述
func describeRoundList(rounds []model.Round) string {
	if len(rounds) == 0 {
		return ""
	}
	parts := make([]string, 0, len(rounds))
	for _, r := range rounds {
		name := r.Name
		if name == "" {
			name = "未命名页签"
		}
		parts = append(parts, name+"(kklxdm="+r.Kklxdm+")")
	}
	return strings.Join(parts, "、")
}

// parseErrorEnvelope 识别服务端业务级错误信封。
//
// 实机结论：PartDisplay 缺 xkkz_xh 时返回 {"flag":"0","msg":"加密串错误"}——
// 它没有 tmpList 字段，会被 ParseCourseList 解析成"0 门课"，把协议错误
// 伪装成正常业务结果（用户以为没排课，实际是参数问题）。
// 只在 flag 明确非 1 且带 msg 时判定为错误信封。
func parseErrorEnvelope(resp string) (string, bool) {
	trimmed := strings.TrimSpace(resp)
	if !strings.HasPrefix(trimmed, "{") {
		return "", false
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return "", false
	}
	flagRaw, hasFlag := raw["flag"]
	if !hasFlag {
		return "", false
	}
	flag, ok := model.ToInt(flagRaw)
	if !ok || flag == 1 {
		return "", false
	}
	msg := firstString(raw, "msg", "message", "message1", "msgText")
	if msg == "" {
		return "", false
	}
	return msg, true
}

// parseHiddenInputs 解析 HTML 中所有 hidden input 的 name/value
// 使用 golang.org/x/net/html 正规解析，与 login.go 中 parseLoginForm 逻辑一致，
// 避免手写字符串切割在多行/属性顺序变化时解析失败的问题。
func (c *Client) parseHiddenInputs(pageHTML string) map[string]string {
	result := make(map[string]string)

	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return result
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "input" {
			attrs := attrMap(n.Attr)
			if strings.EqualFold(attrs["type"], "hidden") {
				if name := attrs["name"]; name != "" {
					result[name] = attrs["value"]
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	return result
}

// buildCourseQueryParams 构建课程查询参数（PartDisplay）
//
// 实机结论（2026-09-15）：表单只提交自己 form 的字段 —— 整包发参数池
// （208 项页面隐藏字段）会被服务端以「系统运行异常」拒绝；浏览器实际提交的
// 是 browserListParams 白名单。因此这里**从参数池按白名单挑字段**，
// 再用轮次整组参数与用户筛选覆盖。
func (c *Client) buildCourseQueryParams(poolParams map[string]string, round model.Round, cfg *model.Config) map[string]string {
	params := pickWhitelisted(poolParams, browserListParamSet)

	// 轮次整组参数（与 Display/选课提交同源，混用会得到误导性的「加密串错误」）
	if round.Kklxdm != "" {
		params["kklxdm"] = round.Kklxdm
	}
	if round.XkkzID != "" {
		params["xkkz_id"] = round.XkkzID
	}
	// xkkz_xh 是必需加密串（首页 firstXkkzXh，248 字符 hex）：
	// PartDisplay 缺它返回 {"flag":"0","msg":"加密串错误"}。
	// 它不在 47 项白名单里，是实机证明的必需补充项。
	if round.XkkzXh != "" {
		params["xkkz_xh"] = round.XkkzXh
	}
	if round.NjdmID != "" {
		params["njdm_id"] = round.NjdmID
	}
	if round.ZyhID != "" {
		params["zyh_id"] = round.ZyhID
	}

	// 课程分类（多选）：把 UI 勾选的中文标签映射为页面里真实的 kcgs_list 取值。
	// 修复（V3.0 bug）：cfg.Categories 此前只被收集、从未参与查询 ——
	// 用户勾选"体育类"完全没有效果，属于静默失效。
	// 浏览器多选提交格式为 kcgs_list[0]=...&kcgs_list[1]=...（实机抓包）。
	if codes := c.categoryCodes(cfg); len(codes) > 0 {
		for i, code := range codes {
			params[fmt.Sprintf("kcgs_list[%d]", i)] = code
		}
	}

	return params
}

// courseListCacheKey 生成课程列表缓存的键（只包含影响查询结果的字段）
func (c *Client) courseListCacheKey(cfg *model.Config) string {
	labels := make([]string, 0, len(cfg.Categories))
	for label, on := range cfg.Categories {
		if on {
			labels = append(labels, label)
		}
	}
	sort.Strings(labels)
	return model.NormalizeCourseType(cfg.CourseType) + "|" + strings.Join(labels, ",")
}

// categoryCodes 把用户勾选的分类标签映射为页面里的选项值
//
// 只在页面确实提供了同名选项时才返回，避免向服务端提交臆造的类别代码。
func (c *Client) categoryCodes(cfg *model.Config) []string {
	if len(cfg.Categories) == 0 {
		return nil
	}
	opts := c.CategoryOptions()
	if len(opts) == 0 {
		return nil
	}
	codes := make([]string, 0, len(cfg.Categories))
	for label, on := range cfg.Categories {
		if !on {
			continue
		}
		if v, ok := opts[label]; ok && v != "" {
			codes = append(codes, v)
		}
	}
	sort.Strings(codes)
	return codes
}

// MatchCategories 检查用户勾选的分类能否在页面选项中匹配
//
// 返回值分别是被采纳的标签与未匹配的标签，用于向用户明确说明"哪些分类没生效"，
// 而不是让筛选条件静默失效。
func (c *Client) MatchCategories(labels map[string]bool) (applied, unmatched []string) {
	opts := c.CategoryOptions()
	for label, on := range labels {
		if !on {
			continue
		}
		if _, ok := opts[label]; ok {
			applied = append(applied, label)
		} else {
			unmatched = append(unmatched, label)
		}
	}
	sort.Strings(applied)
	sort.Strings(unmatched)
	return applied, unmatched
}

// parseCategoryOptions 从选课页面解析"课程归属 / 课程类别"下拉框，
// 返回「选项文本 → 选项值」映射。
//
// 设计取舍：不对 DOM 名称做硬编码断言（学校部署各不相同），改为按名称关键词匹配
// （kcgs / kclb / kkgs / category）；选项值优先取 value 属性，缺失时退化为文本。
// 多页面传入时后者覆盖前者（Display 页通常比首页更完整）。
func parseCategoryOptions(pages ...string) map[string]string {
	result := make(map[string]string)

	for _, pageHTML := range pages {
		if strings.TrimSpace(pageHTML) == "" {
			continue
		}
		doc, err := html.Parse(strings.NewReader(pageHTML))
		if err != nil {
			continue
		}

		var walk func(*html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "select" {
				name := strings.ToLower(attrMap(n.Attr)["name"])
				if strings.Contains(name, "kcgs") || strings.Contains(name, "kclb") ||
					strings.Contains(name, "kkgs") || strings.Contains(name, "category") {
					for opt := n.FirstChild; opt != nil; opt = opt.NextSibling {
						if opt.Type != html.ElementNode || opt.Data != "option" {
							continue
						}
						text := strings.TrimSpace(nodeText(opt))
						if text == "" {
							continue
						}
						value := strings.TrimSpace(attrMap(opt.Attr)["value"])
						if value == "" {
							value = text
						}
						result[text] = value
					}
				}
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				walk(child)
			}
		}
		walk(doc)
	}

	return result
}

// nodeText 递归收集节点下的全部文本
func nodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return sb.String()
}

// GetClassInfo 获取课程详情（上课时间、do_jxb_id 加密 ID、教师名、容量等）
//
// 修复点（V3.0 bug）：原实现固定按 JSON 对象解析，而同一接口在不同正方部署下
// 可能返回数组（Efarxs 那份"实际抢到过课"的实现就是按 []map 解析的）。形态不符时
// Unmarshal 直接失败，robber 侧会 `continue` 跳过该课 —— 结果是整轮选课颗粒无收。
//
// 现在改为"对象 / 数组"双形态兼容，并对 JSON key 做归一化（忽略大小写与下划线），
// 取不到详情时由调用方降级用短 jxb_id 重试，而不是直接放弃这门课。
//
// 实机补充（2026-10-12）：sfkxk=0（本轮不可选课）时详情接口返回
// **HTTP 200 + 裸 JSON 字符串 "0"**（无教学班）。直接按数组/对象解析会报
// 难懂的 `cannot unmarshal string into ...`，掩盖真实原因 —— 先识别裸 "0"/"1"。
func (c *Client) GetClassInfo(course *model.Course) (*model.CourseExtra, error) {
	// 详情接口同样按白名单取参（整包参数池会被服务端拒绝）
	params := pickWhitelisted(c.SelectInitParams(), browserInfoParamSet)
	params["kch_id"] = course.ID
	if round, _ := c.roundFor(course.Type); round.XkkzXh != "" {
		params["xkkz_xh"] = round.XkkzXh
	}

	infoURL := c.buildURL(pathCourseInfo) + "?gnmkdm=" + gnmkdmSelect
	resp, err := c.doPost(infoURL, params)
	if err != nil {
		return nil, err
	}

	// 裸 "0"/"1" 响应：sfkxk=0 时服务端不返回教学班数据（不是解析失败）
	if trimmed := strings.TrimSpace(resp); trimmed == `"0"` || trimmed == `"1"` || trimmed == "0" || trimmed == "1" {
		return nil, fmt.Errorf("课程详情接口返回裸 %s：本轮不可选课(sfkxk=0)或课程暂无教学班数据，服务端不返回详情", trimmed)
	}

	extra := parseCourseExtra(resp)
	if extra == nil {
		return nil, fmt.Errorf("解析课程详情失败：响应既不是对象也不是数组（前120字符: %s）", previewOf(resp, 120))
	}
	return extra, nil
}

// normalizeJSONKey 归一化 JSON key：转小写并去掉下划线
// 用于兼容 do_jxb_id / doJxbId / dojxbid 等不同写法
func normalizeJSONKey(k string) string {
	k = strings.ToLower(k)
	return strings.ReplaceAll(k, "_", "")
}

// extraFromMap 从一个 JSON 对象中提取课程详情字段
func extraFromMap(obj map[string]interface{}) *model.CourseExtra {
	if len(obj) == 0 {
		return nil
	}

	norm := make(map[string]interface{}, len(obj))
	for k, v := range obj {
		norm[normalizeJSONKey(k)] = v
	}

	get := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := norm[normalizeJSONKey(k)]; ok {
				if s, ok := model.ToString(v); ok && s != "" {
					return s
				}
			}
		}
		return ""
	}

	doJxb := get("do_jxb_id", "doJxbId")
	kcmc := get("kcmc")
	if doJxb == "" && kcmc == "" {
		return nil
	}

	// 容量/已选：jxbrl/jxbrs（详情接口字段；列表接口没有容量字段）
	capacity, _ := model.ToInt(obj["jxbrl"])
	selected, _ := model.ToInt(obj["jxbrs"])

	return &model.CourseExtra{
		ClassInfo: get("jsmc"),
		ExamInfo:  get("sksj"),
		Remark:    get("kcbj"),
		DoJxbID:   doJxb,
		// 教师名只在详情里（列表接口 288/288 无 jsm/jsxx 字段），
		// jsxx 形如 "工号/姓名/职称"，三格式解析见 model.ParseTeacherFromJsxx
		Teacher: model.ParseTeacherFromJsxx(get("jsxx")),
		// 容量以服务端实时值为准；本地缓存不得用于阻止提交
		Capacity: capacity,
		Selected: selected,
	}
}

// parseCourseExtra 解析课程详情响应，兼容数组与对象两种形态
func parseCourseExtra(resp string) *model.CourseExtra {
	trimmed := strings.TrimSpace(resp)
	if trimmed == "" {
		return nil
	}

	// 形态 A：数组 —— 取第一个能解析出内容的对象元素
	if strings.HasPrefix(trimmed, "[") {
		var arr []interface{}
		if err := json.Unmarshal([]byte(trimmed), &arr); err == nil {
			for _, el := range arr {
				if m, ok := el.(map[string]interface{}); ok {
					if extra := extraFromMap(m); extra != nil {
						return extra
					}
				}
			}
		}
		return nil
	}

	// 形态 B：对象
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return nil
	}
	return extraFromMap(obj)
}

// previewOf 截取字符串前 n 个字符，用于错误信息展示
func previewOf(s string, n int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= n {
		return string(runes)
	}
	return string(runes[:n]) + "..."
}
