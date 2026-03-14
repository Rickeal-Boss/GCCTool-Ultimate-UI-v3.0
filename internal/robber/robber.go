package robber

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/client"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/pkg/logger"
)

const (
	// baseInterval 每轮请求之间的基础等待时间（毫秒）
	// 加上随机 jitter 后实际范围为 200~500ms，避免多 worker 同步发包形成脉冲
	baseIntervalMs = 200
	// jitterMs 随机抖动上限（毫秒）
	jitterMs = 300

	// rateLimitBackoffBase 被限流（429/503）时的初始退避时间（秒）
	rateLimitBackoffBase = 5
	// rateLimitBackoffMax 被限流时最大退避时间（秒）
	rateLimitBackoffMax = 60
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

	// 取消之前的context（如有）
	if r.cancel != nil {
		r.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	r.logger.Info("开始抢课...")

	// M5：先登录，再等待开始时间
	// 每次 Start 都创建了新的 Client（cookie jar 是空的），必须重新登录
	r.logger.Info("正在登录教务系统...")
	if err := r.client.Login(cfg); err != nil {
		r.running = false
		cancel()
		return fmt.Errorf("登录失败: %w", err)
	}
	r.logger.Info("登录成功")

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

	// backoffSec：当前退避秒数，初始为 0（不退避）
	// 被限流后指数增长，成功后重置
	backoffSec := 0

	for {
		select {
		case <-ctx.Done():
			r.logger.Info(fmt.Sprintf("Worker %d 停止", id))
			return
		default:
			// 如果处于退避状态，先等待后再试
			if backoffSec > 0 {
				r.logger.Warn(fmt.Sprintf("Worker %d 触发限流保护，等待 %ds 后重试", id, backoffSec))
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(backoffSec) * time.Second):
				}
			}

			// 执行抢课
			err := r.robCourse(id)
			if err != nil {
				errMsg := err.Error()
				// 识别服务端限流信号（HTTP 429 / 503 / "频繁"等关键词）
				// 被限流时启用指数退避，避免持续轰炸触发封号
				if isRateLimited(errMsg) {
					if backoffSec == 0 {
						backoffSec = rateLimitBackoffBase
					} else {
						backoffSec = min(backoffSec*2, rateLimitBackoffMax)
					}
					r.logger.Warn(fmt.Sprintf("Worker %d 检测到限流响应: %v", id, err))
				} else {
					// 普通失败：重置退避，记录日志
					backoffSec = 0
					r.logger.Error(fmt.Sprintf("Worker %d 抢课失败: %v", id, err))
				}
			} else {
				// 成功：重置退避
				backoffSec = 0
			}

			// M3：加随机 jitter，防止多 Worker 同步发包形成脉冲流量
			// 实际等待范围：baseIntervalMs ~ baseIntervalMs+jitterMs (200~500ms)
			jitter := rand.Intn(jitterMs)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(baseIntervalMs+jitter) * time.Millisecond):
			}
		}
	}
}

// isRateLimited 判断错误是否为服务端限流
func isRateLimited(msg string) bool {
	keywords := []string{"429", "503", "频繁", "限流", "too many", "rate limit", "slow down"}
	lower := strings.ToLower(msg)
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// min 返回两个整数中较小的那个（Go 1.21 之前无内置 min）
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
