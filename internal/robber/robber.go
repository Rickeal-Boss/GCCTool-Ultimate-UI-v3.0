package robber

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/client"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/stealth"
	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/pkg/logger"
)

// ─────────────────────────────────────────────────────────────────────────────
// 常量配置
// ─────────────────────────────────────────────────────────────────────────────

const (
	// maxReLoginAttempts Session 失效后最大自动重新登录次数
	maxReLoginAttempts = 3

	// rateLimitBackoffBase 初始限流退避（秒）
	rateLimitBackoffBase = 5
	// rateLimitBackoffMax 最大限流退避（秒）
	rateLimitBackoffMax = 60

	// aggressiveThresholdSec 距离开始时间多少秒内切换到激进模式
	aggressiveThresholdSec = 30

	// sessionKeepaliveInterval Session 保活间隔（每隔此时间访问一次首页）
	sessionKeepaliveInterval = 4 * time.Minute
)

// ─────────────────────────────────────────────────────────────────────────────
// Worker 风控状态机
// ─────────────────────────────────────────────────────────────────────────────

// workerState Worker 运行状态（用于反检测策略切换）
type workerState int

const (
	wsNormal       workerState = iota // 正常模式
	wsRateLimited                     // 限流退避中
	wsRelogging                       // 重新登录中
	wsBanned                          // 账号封禁，永久停止
)

// ─────────────────────────────────────────────────────────────────────────────
// Robber 核心结构
// ─────────────────────────────────────────────────────────────────────────────

// Robber 抢课调度器 V3.1
//
// V3.1 新增能力：
//  1. 风控信号分级处理：限流退避 / Session重登 / 封号停止
//  2. 自动重新登录：Session 失效时最多重试 3 次
//  3. 人性化请求节奏：距开抢时间 <30s 切换激进模式，其余时间使用随机化延迟
//  4. Session 保活：等待期间每 4 分钟轻量 GET 一次，防止 Session 超时
//  5. 详细的风控告警日志：用户可以看到实时的反检测状态
type Robber struct {
	client *client.Client
	logger *logger.Logger
	config *model.Config

	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// 统计计数器
	successCount int
	failCount    int
	reloginCount int
	mu           sync.Mutex
}

// NewRobber 创建抢课调度器
func NewRobber(clt *client.Client, log *logger.Logger) *Robber {
	return &Robber{
		client: clt,
		logger: log,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Start / Stop
// ─────────────────────────────────────────────────────────────────────────────

// Start 登录并在后台启动抢课任务（非阻塞）
func (r *Robber) Start(cfg *model.Config) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("抢课已经在运行中")
	}

	r.config = cfg
	r.running = true
	r.successCount = 0
	r.failCount = 0
	r.reloginCount = 0

	// 重置熔断器和退避（新任务从干净状态开始）
	r.client.CircuitBreaker().Reset()
	r.client.BackoffStrategy().Reset()

	if r.cancel != nil {
		r.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.mu.Unlock()

	// 同步登录
	r.logger.Info("正在登录教务系统...")
	if err := r.client.Login(cfg); err != nil {
		r.running = false
		cancel()
		return fmt.Errorf("登录失败: %w", err)
	}
	r.logger.Success("登录成功")

	// 后台调度 goroutine
	go func() {
		startTime := r.calculateStartTime()
		r.logger.Info(fmt.Sprintf("计划开始时间: %s", startTime.Format("15:04:05")))

		// 等待开始时间（期间保活 Session）
		r.waitWithKeepalive(ctx, startTime)

		select {
		case <-ctx.Done():
			r.logger.Info("等待期间任务已取消")
			return
		default:
		}

		r.logger.Info("🚀 开始抢课！")
		r.startWorkers(ctx)
	}()

	return nil
}

// Stop 停止抢课
func (r *Robber) Stop() {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return
	}

	r.logger.Info("正在停止抢课...")
	r.running = false

	if r.cancel != nil {
		r.cancel()
	}
	r.mu.Unlock()

	r.wg.Wait()

	r.mu.Lock()
	sc, fc, rc := r.successCount, r.failCount, r.reloginCount
	r.mu.Unlock()

	r.logger.Info(fmt.Sprintf(
		"抢课已停止 | 成功: %d | 失败: %d | 重新登录: %d | 熔断器: %s",
		sc, fc, rc, r.client.CircuitBreaker().StateName(),
	))
}

// IsRunning 返回当前是否正在运行（线程安全）
func (r *Robber) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

// ─────────────────────────────────────────────────────────────────────────────
// 等待策略
// ─────────────────────────────────────────────────────────────────────────────

// calculateStartTime 计算目标开始时间（含提前量）
func (r *Robber) calculateStartTime() time.Time {
	now := time.Now()
	target := time.Date(
		now.Year(), now.Month(), now.Day(),
		r.config.Hour, r.config.Minute, 0, 0,
		now.Location(),
	)
	target = target.Add(-time.Duration(r.config.Advance) * time.Minute)

	if target.Before(now) {
		target = target.Add(24 * time.Hour)
	}
	return target
}

// waitWithKeepalive 等待到目标时间，期间定期保活 Session
//
// 保活原理：正方系统 Session 一般 10-30 分钟超时，
// 每隔 4 分钟访问一次系统首页（轻量 GET）可重置超时计时器。
// 这避免了"登录成功 → 等待 30 分钟 → Session 过期 → 开始时登录失效"的问题。
func (r *Robber) waitWithKeepalive(ctx context.Context, target time.Time) {
	keepaliveTick := time.NewTicker(sessionKeepaliveInterval)
	defer keepaliveTick.Stop()

	for {
		remaining := time.Until(target)
		if remaining <= 0 {
			return
		}

		// 距开始 <30s 时，退出等待循环（进入激进模式）
		if remaining <= aggressiveThresholdSec*time.Second {
			r.logger.Info(fmt.Sprintf("⚡ 距开始 %.0f 秒，切换激进模式", remaining.Seconds()))
			r.client.SetDelayProfile(stealth.DelayAggressive)
			// 精确等待剩余时间
			select {
			case <-ctx.Done():
				return
			case <-time.After(remaining):
				// Speed-Opt + Anti-Fix: 进入抢课模式（极速模式 + 精准风控）
				r.logger.Info("🚀 开始抢课！进入极速模式（毫秒级延迟 + 只检测账号封禁）")
				r.client.SetRobbingMode(true)
				return
			}
		}

		r.logger.Info(fmt.Sprintf("⏱ 等待 %d 分 %d 秒...", int(remaining.Minutes()), int(remaining.Seconds())%60))

		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second): // 每 10 秒重新检查剩余时间（输出倒计时）
			// 继续循环
		case <-keepaliveTick.C:
			// 执行保活请求
			r.doKeepalive(ctx)
		}
	}
}

// doKeepalive 执行 Session 保活请求（访问系统首页）
func (r *Robber) doKeepalive(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	default:
	}
	r.logger.Info("🔑 Session 保活中...")
	// 访问主页（轻量 GET，不提交任何数据）
	if err := r.client.CheckSessionAlive(); err != nil {
		r.logger.Warn(fmt.Sprintf("Session 保活失败: %v", err))
		// 尝试重新登录
		r.tryRelogin(ctx, 1)
	} else {
		r.logger.Info("✓ Session 保活成功")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Worker 管理
// ─────────────────────────────────────────────────────────────────────────────

// startWorkers 启动指定数量的并发 Worker
//
// Speed-Opt: 使用 WaitGroup 并发启动所有 Worker，确保同时启动
// 避免逐个启动导致的 50~100ms 启动延迟
func (r *Robber) startWorkers(ctx context.Context) {
	r.logger.Info(fmt.Sprintf("🚀 并发启动 %d 个 Worker...", r.config.Threads))

	var wg sync.WaitGroup
	wg.Add(r.config.Threads)

	for i := 0; i < r.config.Threads; i++ {
		go func(id int) {
			defer wg.Done()
			r.worker(ctx, id)
		}(i + 1)
	}

	wg.Wait()
	r.logger.Info("所有 Worker 已完成")

	// Speed-Opt + Anti-Fix: 抢课结束，关闭抢课模式
	r.logger.Info("抢课结束，关闭极速模式")
	r.client.SetRobbingMode(false)
}

// worker 抢课 Worker（V3.1 风控感知版）
//
// 核心风控处理逻辑：
//  1. [风控-停止]  → 立即结束此 Worker，广播停止信号
//  2. [风控-会话]  → 触发重新登录，最多重试 maxReLoginAttempts 次
//  3. [风控-限流]  → 指数退避等待后重试
//  4. 正常失败     → 短暂延迟后继续重试
func (r *Robber) worker(ctx context.Context, id int) {
	defer r.wg.Done()

	r.logger.Info(fmt.Sprintf("Worker %d 启动", id))

	state := wsNormal
	backoffSec := 0
	reloginAttempts := 0

	for {
		// 检查停止信号
		select {
		case <-ctx.Done():
			r.logger.Info(fmt.Sprintf("Worker %d 停止", id))
			return
		default:
		}

		// 熔断器检查（熔断时等待，不是退出）
		if cbErr := r.client.CircuitBreaker().Allow(); cbErr != nil {
			r.logger.Warn(fmt.Sprintf("Worker %d 熔断器阻断: %v", id, cbErr))
			// 等待熔断冷却，期间可响应停止信号
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}

		// 限流退避
		if backoffSec > 0 {
			r.logger.Warn(fmt.Sprintf("Worker %d 退避等待 %ds...", id, backoffSec))
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(backoffSec) * time.Second):
			}
		}

		err := r.robCourse(id)

		if err == nil {
			r.mu.Lock()
			r.successCount++
			r.mu.Unlock()
			backoffSec = 0
			state = wsNormal
			// Speed-Opt + Anti-Fix: 选课成功后立即返回（关闭抢课模式）
			r.logger.Success("✅ 选课成功！停止所有 Worker")
			r.running = false
			r.cancel() // 广播停止信号给所有 Worker
			return
		}

		errMsg := err.Error()
		r.mu.Lock()
		r.failCount++
		r.mu.Unlock()

		switch {
		case isBannedError(errMsg):
			// 账号封禁：立刻停止所有 Worker
			r.logger.Error(fmt.Sprintf("🚨 Worker %d 检测到账号封禁信号！立即停止所有任务！原因: %v", id, err))
			r.running = false
			r.cancel() // 广播停止信号给所有 Worker
			return

		case isSessionError(errMsg):
			// Session 失效：尝试重新登录
			reloginAttempts++
			r.mu.Lock()
			r.reloginCount++
			r.mu.Unlock()
			state = wsRelogging

			if reloginAttempts > maxReLoginAttempts {
				r.logger.Error(fmt.Sprintf("Worker %d Session 失效，重新登录已达上限 %d 次，停止此 Worker", id, maxReLoginAttempts))
				return
			}

			r.logger.Warn(fmt.Sprintf("Worker %d Session 失效，正在重新登录（第 %d/%d 次）...", id, reloginAttempts, maxReLoginAttempts))
			if loginErr := r.tryRelogin(ctx, reloginAttempts); loginErr != nil {
				r.logger.Error(fmt.Sprintf("Worker %d 重新登录失败: %v", id, loginErr))
				// 短暂等待后再试
				select {
				case <-ctx.Done():
					return
				case <-time.After(3 * time.Second):
				}
			} else {
				r.logger.Success(fmt.Sprintf("Worker %d 重新登录成功", id))
				reloginAttempts = 0
				state = wsNormal
			}

		case isRateLimitError(errMsg):
			// 限流：指数退避
			state = wsRateLimited
			if backoffSec == 0 {
				backoffSec = rateLimitBackoffBase
			} else {
				backoffSec = min(backoffSec*2, rateLimitBackoffMax)
			}
			r.logger.Warn(fmt.Sprintf("Worker %d 触发限流，退避 %ds: %v", id, backoffSec, err))

		default:
			// 普通失败：重置退避，短暂等待后继续
			backoffSec = 0
			state = wsNormal
			r.logger.Error(fmt.Sprintf("Worker %d 抢课失败: %v", id, err))
		}

		// 每轮结束后随机延迟（反检测：避免机械均匀间隔）
		_ = state // 未来可根据 state 进一步差异化延迟
		delay := stealth.JitteredDelay(r.client.DelayProfile())
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 抢课逻辑
// ─────────────────────────────────────────────────────────────────────────────

// robCourse 执行一轮抢课逻辑
func (r *Robber) robCourse(workerID int) error {
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

		if course.Extra == nil {
			extra, err := r.client.GetClassInfo(course.ID)
			if err != nil {
				r.logger.Warn(fmt.Sprintf("获取课程详情失败: %s - %v", course.Name, err))
				continue
			}
			course.Extra = extra
		}

		r.logger.Info(fmt.Sprintf("Worker %d 尝试选课: %s (%s)", workerID, course.Name, course.Teacher))
		if err := r.client.SelectCourse(course); err != nil {
			r.logger.Warn(fmt.Sprintf("选课失败: %s - %v", course.Name, err))
			continue
		}

		r.logger.Success(fmt.Sprintf("✓ 选课成功: %s (%s) %s", course.Name, course.Teacher, course.WeekTime))
		return nil
	}

	return fmt.Errorf("本轮未能成功选到课程")
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

// ─────────────────────────────────────────────────────────────────────────────
// 重新登录
// ─────────────────────────────────────────────────────────────────────────────

// tryRelogin 尝试重新登录（带指数退避）
func (r *Robber) tryRelogin(ctx context.Context, attempt int) error {
	// 重登前等待一下，防止连续快速重登被检测
	waitSec := attempt * 2
	r.logger.Info(fmt.Sprintf("等待 %ds 后重新登录...", waitSec))
	select {
	case <-ctx.Done():
		return fmt.Errorf("任务已取消")
	case <-time.After(time.Duration(waitSec) * time.Second):
	}

	if err := r.client.Login(r.config); err != nil {
		return fmt.Errorf("重新登录失败: %w", err)
	}

	// 重置熔断器（重新登录后熔断器也应该复位）
	r.client.CircuitBreaker().Reset()
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// 错误分类辅助
// ─────────────────────────────────────────────────────────────────────────────

// isBannedError 判断是否为账号封禁错误
func isBannedError(msg string) bool {
	lower := strings.ToLower(msg)
	for _, kw := range []string{"风控-停止", "账号已被封禁", "账号锁定", "账号封禁", "暂停使用"} {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// isSessionError 判断是否为 Session 失效错误
func isSessionError(msg string) bool {
	lower := strings.ToLower(msg)
	for _, kw := range []string{"风控-会话", "session", "重新登录", "登录超时", "会话"} {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// isRateLimitError 判断是否为限流错误
func isRateLimitError(msg string) bool {
	lower := strings.ToLower(msg)
	for _, kw := range []string{"风控-限流", "429", "503", "频繁", "限流", "too many", "rate limit", "slow down"} {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────────────
// 辅助函数
// ─────────────────────────────────────────────────────────────────────────────

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
