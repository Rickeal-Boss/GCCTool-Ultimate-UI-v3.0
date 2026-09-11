package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/net/html"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/stealth"
)

// GetClassList 获取课程列表
//
// 关键修复：先用不跟随重定向的方式探测选课首页，
// 明确区分 "Session失效(302跳转)" 和 "选课未开放(200正常页面)"，
// 避免系统维护/未开放页面被误判为 Session 失效。
//
// 另外：解析完选课首页与 Display 页后，会把服务端下发的隐藏参数整体缓存到
// Client.selectInitParams，供 SelectCourse 回填（见 client.go 的注释说明）。
func (c *Client) GetClassList(cfg *model.Config) (*model.CourseList, error) {
	indexURL := c.buildURL(pathSelectIndex) + "?gnmkdm=" + gnmkdmSelect + "&layout=default"

	// 步骤0: 用不跟随重定向的探测请求判断 Session 状态
	// 原因：http.Client 默认跟随302，会自动跳到登录页，导致拿到登录页HTML后被误判为Session失效
	// 修复：先单独发一个不跟随重定向的请求，只看 HTTP 状态码
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

	postData1 := c.parseHiddenInputs(indexHTML)
	postData1["gnmkdm"] = gnmkdmSelect

	// 步骤2: 获取选课参数
	// Display 页会下发真正的选课开关参数（xkkz_id 等），这些参数在 SelectCourse
	// 提交时必须原样带上，因此这里不能只把响应丢掉，要解析出来缓存。
	displayURL := c.buildURL(pathSelectDisplay) + "?gnmkdm=" + gnmkdmSelect
	displayHTML, err := c.doPost(displayURL, postData1)
	if err != nil {
		return nil, err
	}

	// 步骤3: 合并"选课首页 + Display 页"下发的隐藏参数并缓存
	initParams := make(map[string]string, len(postData1)+16)
	for k, v := range postData1 {
		initParams[k] = v
	}
	for k, v := range c.parseHiddenInputs(displayHTML) {
		initParams[k] = v
	}
	// 记录本次实际使用的类别码，便于排查"学校改了类别代码导致查不到课"
	initParams["kklxdm"] = getCourseTypeCode(cfg.CourseType)
	c.SetSelectInitParams(initParams)

	// 步骤4: 构建课程查询参数
	postData2 := c.buildCourseQueryParams(initParams, cfg)

	// 步骤5: 获取课程列表JSON
	partURL := c.buildURL(pathCourseList) + "?gnmkdm=" + gnmkdmSelect
	resp, err := c.doPost(partURL, postData2)
	if err != nil {
		return nil, err
	}

	// 步骤6: 解析课程列表（0 门课属于正常业务结果，不返回 error）
	return model.ParseCourseList([]byte(resp))
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

// buildCourseQueryParams 构建课程查询参数
func (c *Client) buildCourseQueryParams(baseParams map[string]string, cfg *model.Config) map[string]string {
	params := make(map[string]string)

	// 复制基础参数
	for k, v := range baseParams {
		params[k] = v
	}

	// 设置查询参数（kklxdm 取归一化后的类别码，避免中文标签导致查询类别错误）
	params["kklxdm"] = getCourseTypeCode(cfg.CourseType)
	params["kch_id"] = "" // 如果指定课程号
	params["jxb_id"] = "" // 如果指定教学班
	params["skbj"] = ""   // 上课班级
	params["sj"] = ""     // 时间

	return params
}

// getCourseTypeCode 获取课程类型对应的正方 kklxdm 查询参数
//
// 统一走 model 层映射：V3.0 里 client 与 model.Course.Match() 各写了一份
// 类型→代码的对应关系，UI 传中文时两边同时失效。现在只有一个来源。
func getCourseTypeCode(courseType string) string {
	return model.CourseTypeCodeKklxdm(courseType)
}

// GetClassInfo 获取课程详情（上课时间、do_jxb_id 加密 ID 等）
//
// 修复点（V3.0 bug）：原实现固定按 JSON 对象解析，而同一接口在不同正方部署下
// 可能返回数组（Efarxs 那份"实际抢到过课"的实现就是按 []map 解析的）。形态不符时
// Unmarshal 直接失败，robber 侧会 `continue` 跳过该课 —— 结果是整轮选课颗粒无收。
//
// 现在改为"对象 / 数组"双形态兼容，并对 JSON key 做归一化（忽略大小写与下划线），
// 取不到详情时由调用方降级用短 jxb_id 重试，而不是直接放弃这门课。
func (c *Client) GetClassInfo(courseID string) (*model.CourseExtra, error) {
	// 详情接口同样需要选课初始化参数，合并缓存后再提交
	params := c.SelectInitParams()
	if params == nil {
		params = make(map[string]string, 2)
	}
	params["kch_id"] = courseID
	params["gnmkdm"] = gnmkdmSelect

	infoURL := c.buildURL(pathCourseInfo) + "?gnmkdm=" + gnmkdmSelect
	resp, err := c.doPost(infoURL, params)
	if err != nil {
		return nil, err
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

	return &model.CourseExtra{
		ClassInfo: get("jsmc"),
		ExamInfo:  get("sksj"),
		Remark:    get("kcbj"),
		DoJxbID:   doJxb,
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
