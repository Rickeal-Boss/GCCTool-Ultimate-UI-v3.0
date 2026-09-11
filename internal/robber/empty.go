package robber

import (
	"errors"
	"fmt"
)

// EmptyKind 空结果的具体成因
type EmptyKind int

const (
	// EmptyNoCourseAtServer 服务端在本次查询条件下返回 0 门课（接口正常，不是故障）
	EmptyNoCourseAtServer EmptyKind = iota
	// EmptyFilteredOut 服务端返回了课程，但没有任何一门命中筛选条件
	EmptyFilteredOut
	// EmptyAllFull 有课程且命中筛选，但全部满员
	EmptyAllFull
)

// EmptyResultError 表示"本轮没有可提交的课程"这一业务结果。
//
// 修复（V3.0 bug）：原实现用一句 fmt.Errorf("没有符合条件的课程") 覆盖了
// 「服务端 0 门 / 有课但没命中筛选 / 有课但全满员」三种完全不同的状态，
// 且都落到 worker 的 default 分支按 Error 级 + 极速节奏刷屏。
// 三者的正确节奏恰好相反：满员要死等（随时有人退课），没排课等也没用。
type EmptyResultError struct {
	Kind    EmptyKind
	Total   int // 服务端返回的课程总数
	Matched int // 命中筛选条件的课程数
}

func (e *EmptyResultError) Error() string {
	switch e.Kind {
	case EmptyNoCourseAtServer:
		return "教务系统当前查询条件下返回 0 门课程（接口正常，本类别可能未排课）"
	case EmptyFilteredOut:
		return fmt.Sprintf("服务端返回 %d 门课，但没有一门命中筛选条件，请检查课程类型/名称/教师/课号/学分", e.Total)
	default:
		return fmt.Sprintf("%d 门候选课程全部满员，等待他人退课", e.Matched)
	}
}

// errRoundNoSelection 本轮有候选课程、也真实提交了选课请求，但都没成功。
// 属于正常轮询中的一轮，不是致命错误，用 Info 级别记录、按正常节奏继续。
var errRoundNoSelection = errors.New("本轮未抢到课程，继续轮询")
