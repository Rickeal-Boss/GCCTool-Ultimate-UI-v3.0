package robber

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/client"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/pkg/logger"
)

const (
	// baseIntervalMs 每轮选课请求之间的基础等待时间（毫秒）
	// 加上随机 jitter 后实际范围 200~500ms，避免多 Worker 同步发包形成脉冲
	baseIntervalMs = 200
	// jitterMs 随机抖动上限（毫秒）
	jitterMs = 300

	// rateLimitBackoffBase 被限流（429/503/"频繁"）时的初始退避秒数
	rateLimitBackoffBase = 5
	// rateLimitBackoffMax 被限流时的最大退避秒数
	rateLimitBackoffMax = 60
)

// Robber 抢课调度器
type Robber struct {
	client *client.Client
	logger *logger.Logger
	config *model.Config

	running int32 // atomic: 0=stopped, 1=running
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// extraCache 缓存 courseID → CourseExtra，避免每轮都重新请求 GetClassInfo
	extraCache map[string]*model.CourseExtra
	extraMu    sync.RWMutex
}

// NewRobber 创建抢课调度器
func NewRobber(clt *client.Client, log *logger.Logger) *Robber {
	return &Robber{
		client:     clt,
		logger:     log,
		extraCache: make(map[string]*model.CourseExtra),
	}
}

// Start 登录并在后台启动抢课任务，立即返回（非阻塞）
//
// 调用方（ui/main.go）已在 go func() 中调用，此处不应再调用 time.Sleep 或 wg.Wait，
// 否则会把 UI goroutine 永久挂起，导致停止按钮无响应。
//
// 流程：
//  1. 同步登录（有错误立即返回，让 UI 显示错误信息）
//  2. 启动独立 goroutine 等待开始时间，时间到后启动 Worker
func (r *Robber) Start(cfg *model.Config) error {
	if !atomic.CompareAndSwapInt32(&r.running, 0, 1) {
		return fmt.Errorf("抢课已经在运行中")
	}

	r.config = cfg

	if r.cancel != nil {
		r.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	// 步骤1：同步登录（每次 Start 都新建了 Client，Cookie Jar 为空，必须重新登录）
	r.logger.Info("正在登录教务系统...")
	if err := r.client.Login(cfg); err != nil {
		atomic.StoreInt32(&r.running, 0)
		cancel()
		return fmt.Errorf("登录失败: %w", err)
	}
	r.logger.Info("登录成功")

	// 步骤2：在后台 goroutine 里等待开始时间，到点再启动 Worker
	// 不在此处阻塞，UI 线程可以继续响应停止按钮
	go func() {
		startTime := r.calculateStartTime()
		r.logger.Info(fmt.Sprintf("计划开始时间: %s", startTime.Format("15:04:05")))
		r.waitUntil(ctx, startTime)

		// 检查是否已被停止（等待期间用户可能点了停止）
		select {
		case <-ctx.Done():
			r.logger.Info("等待期间任务已取消")
			return
		default:
		}

		r.logger.Info("开始抢课...")
		r.startWorkers(ctx)
	}()

	return nil
}

// Stop 停止抢课
//
// 仅发送取消信号，不阻塞等待 Worker 退出。
// Worker 会在下一轮循环检测到 ctx.Done() 后自行退出，
// 避免在 UI 线程中调用 wg.Wait() 导致界面冻结。
func (r *Robber) Stop() {
	if !atomic.CompareAndSwapInt32(&r.running, 1, 0) {
		return
	}

	r.logger.Info("正在停止抢课...")

	if r.cancel != nil {
		r.cancel()
	}

	// 在后台等待 Worker 退出，完成后记录日志
	go func() {
		r.wg.Wait()
		r.logger.Info("抢课已停止")
	}()
}

// calculateStartTime 计算目标开始时间（含提前量）
func (r *Robber) calculateStartTime() time.Time {
	now := time.Now()
	target := time.Date(
		now.Year(), now.Month(), now.Day(),
		r.config.Hour, r.config.Minute, 0, 0,
		now.Location(),
	)

	target = target.Add(-time.Duration(r.config.Advance) * time.Minute)

	// 目标时间已过，改为明天同一时刻
	if target.Before(now) {
		target = target.Add(24 * time.Hour)
	}

	return target
}

// waitUntil 可中断地等待到目标时间
//
// 接受 ctx，用户点停止时可立即退出，不会卡在 time.Sleep 直到开始时间
func (r *Robber) waitUntil(ctx context.Context, target time.Time) {
	duration := time.Until(target)
	if duration <= 0 {
		return
	}

	r.logger.Info(fmt.Sprintf("等待 %d 分 %d 秒...", int(duration.Minutes()), int(duration.Seconds())%60))

	select {
	case <-ctx.Done():
	case <-time.After(duration):
	}
}

// startWorkers 启动指定数量的并发 Worker
func (r *Robber) startWorkers(ctx context.Context) {
	for i := 0; i < r.config.Threads; i++ {
		r.wg.Add(1)
		go r.worker(ctx, i+1)
	}
}

// worker 抢课 Worker
//
// 每轮只发一次选课提交请求（do_jxb_id 在启动前缓存好）。
// 被限流时使用指数退避；ctx 取消时立即退出。
func (r *Robber) worker(ctx context.Context, id int) {
	defer r.wg.Done()

	r.logger.Info(fmt.Sprintf("Worker %d 启动", id))

	// backoffSec：当前退避秒数，0 表示正常节奏
	backoffSec := 0

	for {
		select {
		case <-ctx.Done():
			r.logger.Info(fmt.Sprintf("Worker %d 停止", id))
			return
		default:
		}

		// 限流退避：等待后再发请求
		if backoffSec > 0 {
			r.logger.Warn(fmt.Sprintf("Worker %d 限流保护，等待 %ds 后重试", id, backoffSec))
			select {
			case <-ctx.Done():
				r.logger.Info(fmt.Sprintf("Worker %d 停止", id))
				return
			case <-time.After(time.Duration(backoffSec) * time.Second):
			}
		}

		err := r.robCourse(id)
		switch {
		case err == nil:
			backoffSec = 0
		case isRateLimited(err.Error()):
			// 指数退避：5 → 10 → 20 → ... → 60s
			if backoffSec == 0 {
				backoffSec = rateLimitBackoffBase
			} else {
				backoffSec = min(backoffSec*2, rateLimitBackoffMax)
			}
			r.logger.Warn(fmt.Sprintf("Worker %d 检测到限流: %v", id, err))
		default:
			backoffSec = 0
			r.logger.Error(fmt.Sprintf("Worker %d 抢课失败: %v", id, err))
		}

		// 每轮结束后随机延迟，错开多 Worker 的发包时间点
		jitter := rand.Intn(jitterMs)
		select {
		case <-ctx.Done():
			r.logger.Info(fmt.Sprintf("Worker %d 停止", id))
			return
		case <-time.After(time.Duration(baseIntervalMs+jitter) * time.Millisecond):
		}
	}
}

// robCourse 执行一轮抢课逻辑
//
// 请求次数优化：
//   - 每轮只调用 GetClassList（1次）+ SelectCourse（1次/课程）
//   - GetClassInfo 仅在第一次遇到某门课时调用，do_jxb_id 缓存在 r.extraCache 里
//     （不能缓存在 course.Extra，因为 GetClassList 每轮返回全新的 *Course 对象）
//   - SelectCourse 不再重复 GET 首页 + POST display（已移至 client 层缓存）
func (r *Robber) robCourse(workerID int) error {
	// 获取课程列表（含剩余名额信息）
	courseList, err := r.client.GetClassList(r.config)
	if err != nil {
		return fmt.Errorf("获取课程列表失败: %w", err)
	}

	matched := r.filterCourses(courseList)
	if len(matched) == 0 {
		return fmt.Errorf("没有符合条件的课程")
	}

	for _, course := range matched {
		if course.IsFull() {
			r.logger.Warn(fmt.Sprintf("课程已满: %s", course.Name))
			continue
		}

		// do_jxb_id 优先从 Robber 级别的缓存中取，避免每轮重建 Course 对象后缓存失效
		r.extraMu.RLock()
		extra, cached := r.extraCache[course.ID]
		r.extraMu.RUnlock()

		if !cached {
			var fetchErr error
			extra, fetchErr = r.client.GetClassInfo(course.ID)
			if fetchErr != nil {
				r.logger.Warn(fmt.Sprintf("获取课程详情失败: %s - %v", course.Name, fetchErr))
				continue
			}
			r.extraMu.Lock()
			r.extraCache[course.ID] = extra
			r.extraMu.Unlock()
		}
		course.Extra = extra

		r.logger.Info(fmt.Sprintf("Worker %d 尝试选课: %s (%s)", workerID, course.Name, course.Teacher))
		if err := r.client.SelectCourse(course); err != nil {
			r.logger.Warn(fmt.Sprintf("选课失败: %s - %v", course.Name, err))
			continue
		}

		r.logger.Success(fmt.Sprintf("✓ 选课成功: %s (%s) %s", course.Name, course.Teacher, course.WeekTime))
		return nil
	}

	return fmt.Errorf("没有成功选到课程")
}

// filterCourses 筛选符合配置条件的课程
func (r *Robber) filterCourses(courseList *model.CourseList) []*model.Course {
	matched := make([]*model.Course, 0, len(courseList.Items))
	for _, course := range courseList.Items {
		if course.Match(r.config) {
			matched = append(matched, course)
		}
	}
	return matched
}

// isRateLimited 判断错误信息是否为服务端限流信号
func isRateLimited(msg string) bool {
	lower := strings.ToLower(msg)
	for _, kw := range []string{"429", "503", "频繁", "限流", "too many", "rate limit", "slow down"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// IsRunning 返回当前是否正在运行
func (r *Robber) IsRunning() bool {
	return atomic.LoadInt32(&r.running) == 1
}
