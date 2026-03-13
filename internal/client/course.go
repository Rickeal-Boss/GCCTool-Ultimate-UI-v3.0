package client

import (
	"encoding/json"
	"fmt"
	"strings"

	"gcctool/internal/model"
)

// GetClassList 获取课程列表
func (c *Client) GetClassList(cfg *model.Config) (*model.CourseList, error) {
	// 步骤1: 初始化POST参数1
	indexURL := c.buildURL("/xsxk/zzxkyzb_cxZzxkYzbIndex.html")
	indexHTML, err := c.doGet(indexURL)
	if err != nil {
		return nil, err
	}

	postData1 := c.parseHiddenInputs(indexHTML)

	// 步骤2: 获取更多参数
	displayURL := c.buildURL("/xsxk/zzxkyzb_cxZzxkYzbDisplay.html")
	_, err = c.doPost(displayURL, postData1)
	if err != nil {
		return nil, err
	}

	// 步骤3: 构建课程查询参数
	postData2 := c.buildCourseQueryParams(postData1, cfg)

	// 步骤4: 获取课程列表JSON
	partURL := c.buildURL("/xsxk/zzxkyzb_cxZzxkYzbPartDisplay.html")
	resp, err := c.doPost(partURL, postData2)
	if err != nil {
		return nil, err
	}

	// 步骤5: 解析课程列表
	return model.ParseCourseList([]byte(resp))
}

// parseHiddenInputs 解析HTML中的hidden input
// 关键修复：使用proper的HTML解析，不依赖正则
func (c *Client) parseHiddenInputs(html string) map[string]string {
	result := make(map[string]string)

	// 分割HTML行
	lines := strings.Split(html, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 查找input标签
		if !strings.Contains(line, "<input") {
			continue
		}

		// 检查是否为hidden类型
		if !strings.Contains(line, "type=\"hidden\"") && !strings.Contains(line, "type='hidden'") {
			continue
		}

		// 提取name和value
		// 处理多种格式：name="xxx" value="yyy" 或 name='xxx' value='yyy'
		name := c.extractAttr(line, "name")
		value := c.extractAttr(line, "value")

		if name != "" {
			result[name] = value
		}
	}

	return result
}

// extractAttr 提取HTML属性值
func (c *Client) extractAttr(line, attrName string) string {
	// 查找 attrName="
	prefix := attrName + `="`
	startIdx := strings.Index(line, prefix)
	if startIdx == -1 {
		// 尝试单引号
		prefix = attrName + `='`
		startIdx = strings.Index(line, prefix)
		if startIdx == -1 {
			// 尝试无引号格式 name=xxx
			prefix = attrName + "="
			startIdx = strings.Index(line, prefix)
			if startIdx == -1 {
				return ""
			}
			// 无引号格式：查找下一个空格或>
			endIdx := strings.IndexAny(line[startIdx+len(prefix):], " >")
			if endIdx == -1 {
				return line[startIdx+len(prefix):]
			}
			return line[startIdx+len(prefix) : startIdx+len(prefix)+endIdx]
		}
	}

	startIdx += len(prefix)
	endIdx := strings.Index(line[startIdx:], `"`)
	if endIdx == -1 {
		// 单引号结尾
		endIdx = strings.Index(line[startIdx:], `'`)
		if endIdx == -1 {
			return ""
		}
	}

	return line[startIdx : startIdx+endIdx]
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

// GetClassInfo 获取课程详情（上课时间等）
func (c *Client) GetClassInfo(courseID string) (*model.CourseExtra, error) {
	// 构建查询参数
	params := map[string]string{
		"kch_id": courseID,
	}

	// 发送请求
	url := c.buildURL("/xsxk/zzxkyzbjk_cxJxbWithKchZzxkYzb.html")
	resp, err := c.doPost(url, params)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var result struct {
		Kcmc    string `json:"kcmc"`    // 课程名称
		Jsm     string `json:"jsm"`     // 老师姓名
		Jsmc    string `json:"jsmc"`    // 教室名称
		Sksj    string `json:"sksj"`    // 上课时间
		Kcbj    string `json:"kcbj"`    // 课程备注
	}

	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return nil, fmt.Errorf("解析课程详情失败: %w", err)
	}

	return &model.CourseExtra{
		ClassInfo: result.Jsmc,
		ExamInfo:  result.Sksj,
		Remark:    result.Kcbj,
	}, nil
}
