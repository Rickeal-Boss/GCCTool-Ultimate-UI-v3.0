package client

import (
	"encoding/json"
	"fmt"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
)

// SelectCourse 选课（正方 V9）
//
// 请求次数：仅发 1 次 POST 到选课提交接口。
// 初始化参数（xkkz_id 等）由 GetClassList 调用时已从选课首页提取并缓存在 course 对象中，
// 此处不再重复 GET 首页 + POST display，避免每次选课发 3~4 个请求。
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
// 正方 V9 必须使用 do_jxb_id（加密长 ID），存于 course.Extra.DoJxbID，
// 该值由 robber.go 在调用 SelectCourse 之前通过 GetClassInfo 获取并缓存。
func (c *Client) buildSelectParams(course *model.Course) map[string]string {
	params := map[string]string{
		"kch_id":  course.ID,
		"kcmc":    course.Name,
		"kklxdm":  course.Type,
		"gnmkdm":  gnmkdmSelect,
	}

	// do_jxb_id：V9 使用加密长 ID；Extra 未取到时降级使用短 jxb_id
	if course.Extra != nil && course.Extra.DoJxbID != "" {
		params["jxb_ids"] = course.Extra.DoJxbID
	} else {
		params["jxb_ids"] = course.ClassID
	}

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

	if result.Flag == "1" {
		return nil
	}

	return fmt.Errorf("选课失败: %s", result.Message)
}

// QuerySelectedCourse 查询已选课程
func (c *Client) QuerySelectedCourse() (*model.CourseList, error) {
	queryURL := c.buildURL(pathSelectedCourses) + "?gnmkdm=" + gnmkdmSelect

	params := map[string]string{
		"flag":    "1",
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
		"gnmkdm": gnmkdmSelect,
	}

	resp, err := c.doPost(cancelURL, params)
	if err != nil {
		return err
	}

	return c.parseSelectResult(resp)
}
