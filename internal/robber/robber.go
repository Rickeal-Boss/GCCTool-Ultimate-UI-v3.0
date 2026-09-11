package robber

import (
	"context"
	"errors"
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

	// emptyLogEvery 连续空结果的日志节流间隔（每 N 次空结果才打印一次）
	emptyLogEvery = 100
	// emptySlowDownAfter 连续"服务端 0 门"超过此次数后，把重试节奏从快速降到慢速
	emptySlowDownAfter = 10
	// emptyNoCourseHintAt 连续"服务端 0 门"达到此次数时给出一次醒目提示（即"该放手了"的度）
	emptyNoCourseHintAt = 600
	// emptyWarningMaxRepeats "筛选条件全不命中"的警告最多重复次数（大概率是配置错误）
	emptyWarningMaxRepeats = 3
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
//  6. 课程信息保存：抢课失败时保存已获取的课程信息，供用户手动接管
//
// V3.2 修复：
//  7. 空结果语义拆分（0 门课 / 未命中筛选 / 全满员）并分别给节奏，见 EmptyResultError
//  8. WaitGroup 不再被并发重置（原实现在 Stop() 等待期间重新赋值，存在数据竞争）
type Robber struct {
	client *client.Client
	logger *logger.Logger
	config *model.Config

	running bool
	cancel  context.CancelFunc
	// wg 每次任务创建新的 WaitGroup 实例并保持指针稳定。
	// 原实现在 startWorkers 里直接给字段赋新值，而 Stop() 在释放锁之后才 Wait，
	// 二者并发会触发 data race，极端情况下 panic: WaitGroup is reused before
	// previous Wait has returned。
	wg *sync.WaitGroup

	// 统计计数器
	successCount int
	failCount    int
	reloginCount int
	mu           sync.Mutex

	// 空结果计数（用于区分"没排课"与"满员"的重试节奏）
	emptyNoCourseStreak int // 连续"服务端 0 门"次数
	emptyFilteredWarned int // "筛选全不命中"已提示次数
	emptyFullCounter    int // "全部满员"日志节流计数

	// categoryLogged 是否已提示过课程分类的生效情况（只提示一次，避免刷屏）
	categoryLogged bool

	// 课程信息保存（用于手动接管）
	lastCourseList *model.CourseList
	lastMatched    []*model.Course
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
	r.emptyNoCourseStreak = 0
	r.emptyFilteredWarned = 0
	r.emptyFullCounter = 0
	// 清空上一次任务残留的课程快照，避免"手动接管"面板显示过期数据
	r.lastCourseList = nil
	r.lastMatched = nil
	r.categoryLogged = false
	r.wg = &sync.WaitGroup{} // 新任务使用全新的 WaitGroup

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
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
		cancel()
		return fmt.Errorf("登录失败: %w", err)
	}
	r.logger.Success("登录成功")

	// 后台调度 goroutine
	go func() {
		startTime, rolled := r.calculateStartTime()
		if rolled {
			r.logger.Warn(fmt.Sprintf(
				"设定的开始时间（%02d:%02d，提前 %d 分钟）已过，已自动顺延到明天同一时刻；如需立即开始请把时间改到当前之后",
				r.config.Hour, r.config.Minute, r.config.Advance,
			))
		}
		r.logger.Info(fmt.Sprintf("计划开始时间: %s", startTime.Format("2006-01-02 15:04:05")))

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

// markStopped 标记任务已停止并广播取消信号（线程安全）
//
// 原实现直接在 worker 里写 r.running = false 并调用 r.cancel()，
// 与 Stop() 形成无保护的并发读写。
func (r *Robber) markStopped() {
	r.mu.Lock()
	r.running = false
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Stop 停止抢课
func (r *Robber) Stop() {
	// Anti-Fix-Bug: 添加 nil 检查，防止崩溃
	if r == nil {
		return
	}

	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return
	}
	r.running = false
	if r.cancel != nil {
		r.cancel()
	}
	// 在持锁状态下取出本次任务的 WaitGroup 实例，之后在锁外等待，
	// 避免与 startWorkers 的赋值竞争。
	wg := r.wg
	r.mu.Unlock()

	r.logger.Info("正在停止抢课...")

	if wg != nil {
		wg.Wait()
	}

	r.mu.Lock()
	sc, fc, rc := r.successCount, r.failCount, r.reloginCount
	r.mu.Unlock()

	// Anti-Fix-Bug: 添加 nil 检查
	cbState := "未知"
	if r.client != nil && r.client.CircuitBreaker() != nil {
		cbState = r.client.CircuitBreaker().StateName()
	}

	r.logger.Info(fmt.Sprintf(
		"抢课已停止 | 成功: %d | 失败: %d | 重新登录: %d | 熔断器: %s",
		sc, fc, rc, cbState,
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
//
// 第二个返回值表示是否因"设定时间已过"而顺延到了第二天 —— 原实现静默 +24h，
// 用户会以为程序卡住，因此上层需要据此给出明确提示。
func (r *Robber) calculateStartTime() (time.Time, bool) {
	now := time.Now()
	target := time.Date(
		now.Year(), now.Month(), now.Day(),
		r.config.Hour, r.config.Minute, 0, 0,
		now.Location(),
	)
	target = target.Add(-time.Duration(r.config.Advance) * time.Minute)

	if target.Before(now) {
		return target.Add(24 * time.Hour), true
	}
	return target, false
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
				r.logger.Info("🚀 开始抢课！进入极速模式（毫秒级延迟 + 精准风控）")
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
//
// Anti-Fix-Bug: 不再在此处重置 r.wg（原实现给字段赋新值，与 Stop() 的 Wait 竞争）。
func (r *Robber) startWorkers(ctx context.Context) {
	threads := r.config.Threads

	r.mu.Lock()
	wg := r.wg
	r.mu.Unlock()

	if wg == nil {
		wg = &sync.WaitGroup{}
	}

	r.logger.Info(fmt.Sprintf("🚀 并发启动 %d 个 Worker...", threads))

	// 启动前把实际提交的选课初始化参数名打印一次，便于对照浏览器抓包核对
	// 是否缺少关键开关参数（如 xkkz_id）——这是"选课提交被拒"最难排查的一环。
	if keys := r.client.SelectInitParamKeys(); len(keys) > 0 {
		r.logger.Info(fmt.Sprintf("选课初始化参数（%d 个）: %s", len(keys), strings.Join(keys, ", ")))
	} else {
		r.logger.Warn("未获取到任何选课初始化参数（xkkz_id 等），选课提交很可能被服务端拒绝；" +
			"请确认当前处于选课开放时段，必要时用浏览器抓包核对提交参数")
	}

	wg.Add(threads)
	for i := 0; i < threads; i++ {
		go func(id int) {
			defer wg.Done()
			r.worker(ctx, id)
		}(i + 1)
	}

	wg.Wait()
	r.logger.Info("所有 Worker 已完成")

	// Speed-Opt + Anti-Fix: 抢课结束，关闭抢课模式
	// Anti-Fix-Bug: 添加 nil 检查
	if r.client != nil {
		r.client.SetRobbingMode(false)
	}
}

// worker 抢课 Worker（V3.2 风控感知 + 空结果区分版）
//
// 核心风控处理逻辑：
//  1. [风控-停止]  → 立即结束此 Worker，广播停止信号
//  2. [风控-会话]  → 触发重新登录，最多重试 maxReLoginAttempts 次
//  3. [风控-限流]  → 指数退避等待后重试
//  4. 空结果       → 按成因区分节奏（见 EmptyResultError）
//  5. 待重试(-1)   → 短延迟重试
//  6. 正常失败     → 短暂延迟后继续重试
func (r *Robber) worker(ctx context.Context, id int) {
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

		// Anti-Fix-Bug: 发起请求前再次检查停止信号，避免极速模式下
		// "停止信号已发出但当轮请求已通过检查"导致的滞后日志
		select {
		case <-ctx.Done():
			r.logger.Info(fmt.Sprintf("Worker %d 停止", id))
			return
		default:
		}

		// overrideDelay > 0 时覆盖本轮延迟（用于空结果/待重试等特殊节奏）
		var overrideDelay time.Duration

		err := r.robCourse(id)

		if err == nil {
			r.mu.Lock()
			r.successCount++
			r.emptyNoCourseStreak = 0
			r.emptyFilteredWarned = 0
			r.emptyFullCounter = 0
			r.mu.Unlock()
			backoffSec = 0
			state = wsNormal
			// Speed-Opt + Anti-Fix: 选课成功后立即返回（关闭抢课模式）
			r.logger.Success("✅ 选课成功！停止所有 Worker")
			r.markStopped()
			return
		}

		errMsg := err.Error()
		r.mu.Lock()
		r.failCount++
		r.mu.Unlock()

		var emptyErr *EmptyResultError

		switch {
		case isBannedError(errMsg):
			// 账号封禁：立刻停止所有 Worker
			r.logger.Error(fmt.Sprintf("🚨 Worker %d 检测到账号封禁信号！立即停止所有任务！原因: %v", id, err))
			r.markStopped()
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
				r.logCourseInfoForManualTakeover()
				return
			}

			r.logger.Warn(fmt.Sprintf("Worker %d Session 失效，正在重新登录（第 %d/%d 次）...", id, reloginAttempts, maxReLoginAttempts))
			if loginErr := r.tryRelogin(ctx, reloginAttempts); loginErr != nil {
				// 任务被用户主动取消时，不触发课程信息输出
				if strings.Contains(loginErr.Error(), "任务已取消") {
					return
				}
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

		case errors.Is(err, client.ErrSelectRetry):
			// 服务端要求稍后重试（flag = -1），短延迟后再来，不计入"配置错误"类告警
			backoffSec = 0
			state = wsNormal
			r.logger.Info(fmt.Sprintf("Worker %d 服务端要求重试: %v", id, err))
			overrideDelay = 300 * time.Millisecond

		case errors.As(err, &emptyErr):
			// 三种"没课"状态给完全不同的节奏
			overrideDelay = r.handleEmptyResult(id, emptyErr)

		default:
			// 普通失败：重置退避，短暂等待后继续
			backoffSec = 0
			state = wsNormal
			// 选课未开放/系统维护属于正常等待状态，用 Info 级别而非 Error
			if strings.Contains(errMsg, "选课未开放") || strings.Contains(errMsg, "系统维护") {
				// Anti-Fix-Bug: 选课未开放时强制等待 300ms，避免极速模式下毫秒级狂刷日志
				r.logger.Info(fmt.Sprintf("Worker %d: %v，等待重试...", id, err))
				overrideDelay = 300 * time.Millisecond
			} else if errors.Is(err, errRoundNoSelection) {
				// 有候选课程但都没选上：一轮正常结果，别按错误刷屏
				r.logger.Info(fmt.Sprintf("Worker %d: %v", id, err))
			} else {
				r.logger.Error(fmt.Sprintf("Worker %d 抢课失败: %v", id, err))
			}
		}

		// 每轮结束后随机延迟（反检测：避免机械均匀间隔）
		_ = state // 未来可根据 state 进一步差异化延迟
		delay := overrideDelay
		if delay <= 0 {
			delay = stealth.JitteredDelay(r.client.DelayProfile())
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 空结果处理
// ─────────────────────────────────────────────────────────────────────────────

// onCoursesAvailable 服务端本轮确实返回了课程（Total > 0），重置"0 门课"连击
func (r *Robber) onCoursesAvailable() {
	r.mu.Lock()
	r.emptyNoCourseStreak = 0
	r.emptyFullCounter = 0
	r.mu.Unlock()
}

// logCategoryMatchOnce 首次拿到页面分类选项后，提示一次分类筛选的生效情况
//
// 背景（V3.0 的静默失效）：UI 收集了 9 个课程分类复选框存入 cfg.Categories，
// 但查询逻辑从未使用它们 —— 用户勾了"体育类"没有任何效果，且毫无提示。
// 现在分类会被映射为页面上真实存在的 kcgs_list 取值；无法映射的会明确告警。
func (r *Robber) logCategoryMatchOnce() {
	if len(r.config.Categories) == 0 {
		return
	}

	r.mu.Lock()
	if r.categoryLogged {
		r.mu.Unlock()
		return
	}
	r.categoryLogged = true
	r.mu.Unlock()

	applied, unmatched := r.client.MatchCategories(r.config.Categories)
	if len(applied) > 0 {
		r.logger.Info(fmt.Sprintf("✓ 课程分类已生效（%d 项）: %s", len(applied), strings.Join(applied, ", ")))
	}
	if len(unmatched) > 0 {
		r.logger.Warn(fmt.Sprintf(
			"以下课程分类未能在教务系统页面中找到对应选项，本次未参与筛选：%s（可用浏览器抓包确认 kcgs_list 的真实取值）",
			strings.Join(unmatched, ", "),
		))
	}
}

// handleEmptyResult 依据空结果成因给出不同的日志级别与重试节奏
//
// 返回本轮应等待的时长；返回 0 表示沿用当前档位的抖动延迟（保持极速）。
//
//	| 成因                 | 级别 | 节奏                     | 用户该做什么        |
//	| EmptyNoCourseAtServer| Info | 前 10 轮 500ms，之后 3s  | 切换类别/清空筛选   |
//	| EmptyFilteredOut     | Warn | 3s，最多提示 3 次        | 检查筛选条件        |
//	| EmptyAllFull         | Info | 保持极速（随时有人退课） | 继续挂着等          |
func (r *Robber) handleEmptyResult(workerID int, e *EmptyResultError) time.Duration {
	switch e.Kind {
	case EmptyNoCourseAtServer:
		r.mu.Lock()
		r.emptyNoCourseStreak++
		streak := r.emptyNoCourseStreak
		r.mu.Unlock()

		// 日志节流：首次 + 每 emptyLogEvery 次打印一次，避免多线程毫秒级刷屏
		if streak == 1 || streak%emptyLogEvery == 0 {
			r.logger.Info(fmt.Sprintf("Worker %d: %v（连续第 %d 轮）", workerID, e, streak))
		}
		// 到了该"放手"的度：给一次醒目提示
		if streak == emptyNoCourseHintAt {
			r.logger.Warn("⚠ 已连续多轮查询到 0 门课程：教务系统很可能本轮未开放该类别（如网课）。" +
				"建议切换课程类型/分类，或先到教务系统网页端手动搜索确认，再清空过严的筛选条件。")
		}

		// 前若干轮快速重试（可能是选课刚开放），之后明显放慢，避免对着 0 门课空刷
		if streak <= emptySlowDownAfter {
			return 500 * time.Millisecond
		}
		return 3 * time.Second

	case EmptyFilteredOut:
		// 有课但条件全不命中：大概率是筛选条件/类别码配置错，用 Warn 且最多提示 3 次
		r.mu.Lock()
		r.emptyFilteredWarned++
		warned := r.emptyFilteredWarned
		r.mu.Unlock()

		if warned <= emptyWarningMaxRepeats {
			r.logger.Warn(fmt.Sprintf("Worker %d: %v", workerID, e))
			if warned == emptyWarningMaxRepeats {
				r.logger.Warn(fmt.Sprintf("（该提示最多重复 %d 次，后续不再刷屏；请修正筛选条件后重新启动）", emptyWarningMaxRepeats))
			}
		}
		return 3 * time.Second

	default: // EmptyAllFull
		// 全部满员：保持原有节奏死等（随时可能有人退课），仅低频提示
		r.mu.Lock()
		r.emptyFullCounter++
		n := r.emptyFullCounter
		r.mu.Unlock()

		if n == 1 || n%emptyLogEvery == 0 {
			r.logger.Info(fmt.Sprintf("Worker %d: %v", workerID, e))
		}
		return 0
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 抢课逻辑
// ─────────────────────────────────────────────────────────────────────────────

// robCourse 执行一轮抢课逻辑
//
// 返回值语义：
//   - nil                     → 选课成功
//   - *EmptyResultError       → 本轮无课可提交（区分三种成因）
//   - client.ErrSelectRetry   → 服务端要求稍后重试
//   - errRoundNoSelection     → 有候选课程但都没选上（正常轮询）
//   - 其他                     → 请求/解析层面的错误
func (r *Robber) robCourse(workerID int) error {
	courseList, err := r.client.GetClassList(r.config)
	if err != nil {
		return fmt.Errorf("获取课程列表失败: %w", err)
	}

	// 保存课程列表供手动接管使用
	r.mu.Lock()
	r.lastCourseList = courseList
	r.mu.Unlock()

	// 首次拿到页面下发的分类选项后，提示一次"课程分类筛选是否真的生效"
	r.logCategoryMatchOnce()

	// 0 门课是"业务结果"而非错误：与"接口失败"彻底分开
	if courseList.Total == 0 {
		return &EmptyResultError{Kind: EmptyNoCourseAtServer}
	}
	r.onCoursesAvailable()

	matched := r.filterCourses(courseList)
	if len(matched) == 0 {
		return &EmptyResultError{Kind: EmptyFilteredOut, Total: courseList.Total}
	}

	// 保存匹配的课程列表
	r.mu.Lock()
	r.lastMatched = matched
	r.mu.Unlock()

	attemptable := make([]*model.Course, 0, len(matched))
	for _, course := range matched {
		if course.IsFull() {
			r.logger.Warn(fmt.Sprintf("课程已满: %s", course.Name))
			continue
		}
		attemptable = append(attemptable, course)
	}
	if len(attemptable) == 0 {
		return &EmptyResultError{Kind: EmptyAllFull, Total: courseList.Total, Matched: len(matched)}
	}

	for _, course := range attemptable {
		if course.Extra == nil {
			extra, err := r.client.GetClassInfo(course.ID)
			if err != nil {
				// 修复（V3.0 bug）：详情拿不到不再直接 continue 放弃这门课。
				// 详情接口挂掉不该让整轮选课全废——降级用短 jxb_id 试一次。
				r.logger.Warn(fmt.Sprintf("获取课程详情失败，降级使用短 jxb_id 重试: %s - %v", course.Name, err))
				course.Extra = &model.CourseExtra{}
			} else {
				course.Extra = extra
				if extra.DoJxbID == "" {
					r.logger.Warn(fmt.Sprintf("课程详情未返回 do_jxb_id，降级使用短 jxb_id: %s", course.Name))
				}
			}
		}

		r.logger.Info(fmt.Sprintf("Worker %d 尝试选课: %s (%s)", workerID, course.Name, course.Teacher))
		if err := r.client.SelectCourse(course); err != nil {
			if errors.Is(err, client.ErrSelectRetry) {
				return err // 交由 worker 走"待重试"节奏
			}
			r.logger.Warn(fmt.Sprintf("选课失败: %s - %v", course.Name, err))
			continue
		}

		r.logger.Success(fmt.Sprintf("✓ 选课成功: %s (%s) %s", course.Name, course.Teacher, course.WeekTime))
		return nil
	}

	return errRoundNoSelection
}

// GetLastCourseList 获取最后一次获取的课程列表（用于手动接管）
func (r *Robber) GetLastCourseList() *model.CourseList {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastCourseList
}

// GetLastMatchedCourses 获取最后一次匹配的课程列表（用于手动接管）
func (r *Robber) GetLastMatchedCourses() []*model.Course {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastMatched
}

// ManualSelectCourse 手动选择课程（在应用内直接选课）
func (r *Robber) ManualSelectCourse(course *model.Course) error {
	if r.client == nil {
		return fmt.Errorf("客户端未初始化")
	}

	// 获取课程详情（如果需要）
	if course.Extra == nil || course.Extra.DoJxbID == "" {
		extra, err := r.client.GetClassInfo(course.ID)
		if err != nil {
			return fmt.Errorf("获取课程详情失败: %w", err)
		}
		course.Extra = extra
	}

	// 执行选课
	if err := r.client.SelectCourse(course); err != nil {
		return fmt.Errorf("选课失败: %w", err)
	}

	return nil
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
//
// Anti-Fix-Bug: 使用更精确的关键词，避免误判
// 问题："重新登录"太宽泛，选课未开放页面可能包含"请重新登录"
// 解决：只检测明确的风控信号
func isSessionError(msg string) bool {
	lower := strings.ToLower(msg)
	// 只检测明确的风控信号（由 DetectRisk 生成）
	for _, kw := range []string{"风控-会话", "session过期", "会话已过期", "登录超时"} {
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

// logCourseInfoForManualTakeover 输出课程信息供用户手动接管
func (r *Robber) logCourseInfoForManualTakeover() {
	r.mu.Lock()
	matched := r.lastMatched
	list := r.lastCourseList
	r.mu.Unlock()

	r.logger.Info("═══════════════════════════════════════════════════")
	r.logger.Info("📋 已获取的课程信息（可手动接管选课）")
	r.logger.Info("═══════════════════════════════════════════════════")

	if len(matched) > 0 {
		r.logger.Info(fmt.Sprintf("✓ 匹配条件的课程: %d 门", len(matched)))
		for i, course := range matched {
			if i >= 10 { // 最多显示10门
				r.logger.Info(fmt.Sprintf("  ... 还有 %d 门课程", len(matched)-10))
				break
			}
			capacityInfo := "未知容量"
			if course.Capacity > 0 {
				capacityInfo = fmt.Sprintf("%d/%d", course.Selected, course.Capacity)
			}
			r.logger.Info(fmt.Sprintf("  %d. %s | 教师:%s | 学分:%d | 容量:%s | 时间:%s",
				i+1, course.Name, course.Teacher, course.Credit, capacityInfo, course.WeekTime))
		}
	} else if list != nil && len(list.Items) > 0 {
		r.logger.Info(fmt.Sprintf("✓ 获取到的全部课程: %d 门", len(list.Items)))
		for i, course := range list.Items {
			if i >= 5 { // 最多显示5门
				r.logger.Info(fmt.Sprintf("  ... 还有 %d 门课程", len(list.Items)-5))
				break
			}
			capacityInfo := "未知容量"
			if course.Capacity > 0 {
				capacityInfo = fmt.Sprintf("%d/%d", course.Selected, course.Capacity)
			}
			r.logger.Info(fmt.Sprintf("  %d. %s | 教师:%s | 学分:%d | 容量:%s",
				i+1, course.Name, course.Teacher, course.Credit, capacityInfo))
		}
	} else {
		r.logger.Info("⚠ 暂无课程数据")
	}

	r.logger.Info("───────────────────────────────────────────────────")
	r.logger.Info("📖 手动选课步骤：")
	r.logger.Info("   1. 访问: https://jwxt.gcc.edu.cn/xsxk/zzxkyzb_cxZzxkYzbIndex.html")
	r.logger.Info("   2. 登录教务系统")
	r.logger.Info("   3. 切换到软件的【课程列表】标签页查看详情")
	r.logger.Info("   4. 根据课程信息在教务系统中搜索并选课")
	r.logger.Info("═══════════════════════════════════════════════════")
}
