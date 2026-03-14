package client

import (
	"encoding/json"
	"fmt"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
)

// SelectCourse 选课
func (c *Client) SelectCourse(course *model.Course) error {
	// 步骤1: 获取选课页参数
	indexURL := c.buildURL("/xsxk/zzxkyzb_cxZzxkYzbIndex.html")
	indexHTML, err := c.doGet(indexURL)
	if err != nil {
		return err
	}

	postData1 := c.parseHiddenInputs(indexHTML)

	// 步骤2: 获取更多参数
	displayURL := c.buildURL("/xsxk/zzxkyzb_cxZzxkYzbDisplay.html")
	_, err = c.doPost(displayURL, postData1)
	if err != nil {
		return err
	}

	// 步骤3: 构建选课参数
	postData2 := c.buildSelectParams(postData1, course)

	// 步骤4: 提交选课请求
	selectURL := c.buildURL("/xsxk/zzxkyzb_tjZzxkYzb.html")
	resp, err := c.doPost(selectURL, postData2)
	if err != nil {
		return err
	}

	// 步骤5: 解析响应
	return c.parseSelectResult(resp)
}

// buildSelectParams 构建选课参数
func (c *Client) buildSelectParams(baseParams map[string]string, course *model.Course) map[string]string {
	params := make(map[string]string)

	// 复制基础参数
	for k, v := range baseParams {
		params[k] = v
	}

	// 设置选课参数
	params["kch_id"] = course.ID
	params["jxb_id"] = course.ClassID
	params["kklxdm"] = course.Type

	return params
}

// parseSelectResult 解析选课结果
func (c *Client) parseSelectResult(resp string) error {
	var result struct {
		Flag    string `json:"flag"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return fmt.Errorf("解析选课结果失败: %w", err)
	}

	// 检查flag（成功标志）
	if result.Flag == "1" {
		return nil
	}

	return fmt.Errorf("选课失败: %s", result.Message)
}

// QuerySelectedCourse 查询已选课程
func (c *Client) QuerySelectedCourse() (*model.CourseList, error) {
	// 调用查询已选课程接口
	url := c.buildURL("/xsxk/zzxkyzb_cxYxkAndKc.html")

	// 构建查询参数
	params := map[string]string{
		"flag": "1", // 1=查询已选
	}

	resp, err := c.doPost(url, params)
	if err != nil {
		return nil, err
	}

	return model.ParseCourseList([]byte(resp))
}

// CancelCourse 退课
func (c *Client) CancelCourse(course *model.Course) error {
	// 调用退课接口
	url := c.buildURL("/xsxk/zzxkyzb_tkZzxkYzb.html")

	// 构建退课参数
	params := map[string]string{
		"kch_id": course.ID,
		"jxb_id": course.ClassID,
	}

	resp, err := c.doPost(url, params)
	if err != nil {
		return err
	}

	return c.parseSelectResult(resp)
}
