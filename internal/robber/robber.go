package robber

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gcctool/internal/client"
	"gcctool/internal/model"
	"gcctool/pkg/logger"
)

// Robber 抢课调度器
type Robber struct {
	client  *client.Client
	logger  *logger.Logger
	config  *model.Config

	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewRobber 创建抢课调度器
func NewRobber(clt *client.Client, log *logger.Logger) *Robber {
	return &Robber{
		client: clt,
		logger: log,
	}
}

// Start 开始抢课
func (r *Robber) Start(cfg *model.Config) error {
	if r.running {
		return fmt.Errorf("抢课已经在运行中")
	}

	r.config = cfg
	r.running = true
	r.cancel()

	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	r.logger.Info("开始抢课...")

	// 计算开始时间
	startTime := r.calculateStartTime()
	r.logger.Info(fmt.Sprintf("计划开始时间: %s", startTime.Format("15:04:05")))

	// 等待到开始时间
	r.waitUntil(startTime)

	// 开始抢课循环
	r.startRobbery(ctx)

	return nil
}

// Stop 停止抢课
func (r *Robber) Stop() {
	if !r.running {
		return
	}

	r.logger.Info("正在停止抢课...")
	r.running = false

	if r.cancel != nil {
		r.cancel()
	}

	// 等待所有goroutine结束
	r.wg.Wait()

	r.logger.Info("抢课已停止")
}

// calculateStartTime 计算开始时间
func (r *Robber) calculateStartTime() time.Time {
	now := time.Now()
	target := time.Date(
		now.Year(), now.Month(), now.Day(),
		r.config.Hour, r.config.Minute, 0, 0,
		now.Location(),
	)

	// 提前开始
	target = target.Add(-time.Duration(r.config.Advance) * time.Minute)

	// 如果时间已过，则明天开始
	if target.Before(now) {
		target = target.Add(24 * time.Hour)
	}

	return target
}

// waitUntil 等待到指定时间
func (r *Robber) waitUntil(target time.Time) {
	now := time.Now()
	duration := target.Sub(now)

	if duration > 0 {
		r.logger.Info(fmt.Sprintf("等待 %d 分 %d 秒...", int(duration.Minutes()), int(duration.Seconds())%60))
		time.Sleep(duration)
	}
}

// startRobbery 开始抢课
func (r *Robber) startRobbery(ctx context.Context) {
	// 启动多个worker并发抢课
	for i := 0; i < r.config.Threads; i++ {
		r.wg.Add(1)
		go r.worker(ctx, i+1)
	}
}

// worker 抢课worker
func (r *Robber) worker(ctx context.Context, id int) {
	defer r.wg.Done()

	r.logger.Info(fmt.Sprintf("Worker %d 启动", id))

	for {
		select {
		case <-ctx.Done():
			r.logger.Info(fmt.Sprintf("Worker %d 停止", id))
			return
		default:
			// 执行抢课
			if err := r.robCourse(id); err != nil {
				r.logger.Error(fmt.Sprintf("Worker %d 抢课失败: %v", id, err))
			}

			// 休息一下，避免请求过快
			time.Sleep(time.Duration(100) * time.Millisecond)
		}
	}
}

// robCourse 抢课逻辑
func (r *Robber) robCourse(workerID int) error {
	// 步骤1: 获取课程列表
	courseList, err := r.client.GetClassList(r.config)
	if err != nil {
		return fmt.Errorf("获取课程列表失败: %w", err)
	}

	// 步骤2: 筛选符合条件的课程
	matchedCourses := r.filterCourses(courseList)
	if len(matchedCourses) == 0 {
		return fmt.Errorf("没有符合条件的课程")
	}

	// 步骤3: 遍历课程尝试选课
	for _, course := range matchedCourses {
		// 检查是否满员
		if course.IsFull() {
			r.logger.Warn(fmt.Sprintf("课程已满: %s", course.Name))
			continue
		}

		// 获取课程详情
		extra, err := r.client.GetClassInfo(course.ID)
		if err != nil {
			r.logger.Warn(fmt.Sprintf("获取课程详情失败: %s - %v", course.Name, err))
			continue
		}
		course.Extra = extra

		// 尝试选课
		r.logger.Info(fmt.Sprintf("Worker %d 尝试选课: %s (%s)", workerID, course.Name, course.Teacher))
		err = r.client.SelectCourse(course)
		if err != nil {
			r.logger.Warn(fmt.Sprintf("选课失败: %s - %v", course.Name, err))
			continue
		}

		// 选课成功
		r.logger.Success(fmt.Sprintf("✓ 选课成功: %s (%s) %s", course.Name, course.Teacher, course.WeekTime))
		return nil
	}

	return fmt.Errorf("没有成功选到课程")
}

// filterCourses 筛选课程
func (r *Robber) filterCourses(courseList *model.CourseList) []*model.Course {
	matched := make([]*model.Course, 0)

	for _, course := range courseList.Items {
		// 检查是否匹配配置
		if course.Match(r.config) {
			matched = append(matched, course)
		}
	}

	return matched
}

// IsRunning 检查是否正在运行
func (r *Robber) IsRunning() bool {
	return r.running
}
