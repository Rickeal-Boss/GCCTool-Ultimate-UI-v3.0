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
	displayURL := c.buildURL(pathSelectDisplay) + "?gnmkdm=" + gnmkdmSelect
	_, err = c.doPost(displayURL, postData1)
	if err != nil {
		return nil, err
	}

	// 步骤3: 构建课程查询参数
	postData2 := c.buildCourseQueryParams(postData1, cfg)

	// 步骤4: 获取课程列表JSON
	partURL := c.buildURL(pathCourseList) + "?gnmkdm=" + gnmkdmSelect
	resp, err := c.doPost(partURL, postData2)
	if err != nil {
		return nil, err
	}

	// 步骤5: 解析课程列表
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

	// 设置查询参数
	params["kklxdm"] = getCourseTypeCode(cfg.CourseType)
	params["kch_id"] = "" // 如果指定课程号
	params["jxb_id"] = "" // 如果指定教学班
	params["skbj"] = ""   // 上课班级
	params["sj"] = ""     // 时间

	return params
}

// getCourseTypeCode 获取课程类型代码
func getCourseTypeCode(courseType string) string {
	codes := map[string]string{
		"online": "10", // 网课
		"pe":     "20", // 体育课
		"normal": "30", // 普通课
	}

	if code, ok := codes[courseType]; ok {
		return code
	}
	return "10" // 默认网课
}

// GetClassInfo 获取课程详情（上课时间、do_jxb_id 加密 ID 等）
func (c *Client) GetClassInfo(courseID string) (*model.CourseExtra, error) {
	params := map[string]string{
		"kch_id":  courseID,
		"gnmkdm": gnmkdmSelect,
	}

	infoURL := c.buildURL(pathCourseInfo) + "?gnmkdm=" + gnmkdmSelect
	resp, err := c.doPost(infoURL, params)
	if err != nil {
		return nil, err
	}

	// 解析响应（正方 V9 的课程详情包含 do_jxb_id 加密长 ID）
	var result struct {
		Kcmc    string `json:"kcmc"`      // 课程名称
		Jsm     string `json:"jsm"`       // 老师姓名
		Jsmc    string `json:"jsmc"`      // 教室名称
		Sksj    string `json:"sksj"`      // 上课时间
		Kcbj    string `json:"kcbj"`      // 课程备注
		DoJxbID string `json:"do_jxb_id"` // 正方 V9：加密长 ID，选课必须用此值
	}

	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return nil, fmt.Errorf("解析课程详情失败: %w", err)
	}

	return &model.CourseExtra{
		ClassInfo: result.Jsmc,
		ExamInfo:  result.Sksj,
		Remark:    result.Kcbj,
		DoJxbID:   result.DoJxbID,
	}, nil
}
