package client

import (
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/net/html"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
)

// GetClassList 获取课程列表
func (c *Client) GetClassList(cfg *model.Config) (*model.CourseList, error) {
	// 步骤1: 获取选课首页（携带 gnmkdm 参数，正方 V9 必须）
	indexURL := c.buildURL(pathSelectIndex) + "?gnmkdm=" + gnmkdmSelect + "&layout=default"
	indexHTML, err := c.doGet(indexURL)
	if err != nil {
		return nil, err
	}

	postData1 := c.parseHiddenInputs(indexHTML)
	// 补充 gnmkdm，POST 请求也需要携带
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
//
// 正方 V9 该接口有时返回单对象 {...}，有时返回数组 [{...}]，
// 此处先尝试数组，再 fallback 到单对象，保证两种格式均可正确解析。
func (c *Client) GetClassInfo(courseID string) (*model.CourseExtra, error) {
	params := map[string]string{
		"kch_id": courseID,
		"gnmkdm": gnmkdmSelect,
	}

	infoURL := c.buildURL(pathCourseInfo) + "?gnmkdm=" + gnmkdmSelect
	resp, err := c.doPost(infoURL, params)
	if err != nil {
		return nil, err
	}

	// 统一数据结构（方便两种格式共用解析逻辑）
	type courseDetail struct {
		Kcmc    string `json:"kcmc"`      // 课程名称
		Jsm     string `json:"jsm"`       // 老师姓名
		Jsmc    string `json:"jsmc"`      // 教室名称
		Sksj    string `json:"sksj"`      // 上课时间
		Kcbj    string `json:"kcbj"`      // 课程备注
		DoJxbID string `json:"do_jxb_id"` // 正方 V9：加密长 ID，选课必须用此值
	}

	raw := []byte(resp)
	var detail courseDetail

	// 先尝试数组格式 [{...}]
	var arr []courseDetail
	if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
		detail = arr[0]
	} else if err := json.Unmarshal(raw, &detail); err != nil {
		// 两种格式均解析失败
		return nil, fmt.Errorf("解析课程详情失败: %w", err)
	}

	return &model.CourseExtra{
		ClassInfo: detail.Jsmc,
		ExamInfo:  detail.Sksj,
		Remark:    detail.Kcbj,
		DoJxbID:   detail.DoJxbID,
	}, nil
}
