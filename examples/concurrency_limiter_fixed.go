package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// 令牌桶限流器
type TokenBucketLimiter struct {
	limiter *rate.Limiter
	name    string
}

func NewTokenBucketLimiter(name string, r rate.Limit, b int) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		limiter: rate.NewLimiter(r, b),
		name:    name,
	}
}

func (t *TokenBucketLimiter) Wait(ctx context.Context) error {
	return t.limiter.Wait(ctx)
}

func (t *TokenBucketLimiter) Allow() bool {
	return t.limiter.Allow()
}

// 并发控制器
type ConcurrencyController struct {
	semaphore     chan struct{}
	maxConcurrent int
	name          string
}

func NewConcurrencyController(name string, maxConcurrent int) *ConcurrencyController {
	return &ConcurrencyController{
		semaphore:     make(chan struct{}, maxConcurrent),
		maxConcurrent: maxConcurrent,
		name:          name,
	}
}

func (c *ConcurrencyController) Acquire() {
	c.semaphore <- struct{}{}
}

func (c *ConcurrencyController) Release() {
	<-c.semaphore
}

func (c *ConcurrencyController) TryAcquire() bool {
	select {
	case c.semaphore <- struct{}{}:
		return true
	default:
		return false
	}
}

// 抢课任务管理器
type CourseRobber struct {
	limiter     *TokenBucketLimiter
	concurrency *ConcurrencyController
	taskQueue   chan *RobTask
	workerCount int
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc

	// Bug Fix: 用 sync.Once 保证 Stop 只执行一次，防止重复 close(taskQueue) 引发 panic
	stopOnce sync.Once
}

type RobTask struct {
	CourseID   string
	UserID     string
	Priority   int
	RetryCount int
	MaxRetries int
}

type RobResult struct {
	Task     *RobTask
	Success  bool
	Error    error
	Duration time.Duration
}

func NewCourseRobber(
	rateLimit rate.Limit,
	burst int,
	maxConcurrent int,
	workerCount int,
) *CourseRobber {
	ctx, cancel := context.WithCancel(context.Background())

	return &CourseRobber{
		limiter:     NewTokenBucketLimiter("course_robber", rateLimit, burst),
		concurrency: NewConcurrencyController("robber_workers", maxConcurrent),
		taskQueue:   make(chan *RobTask, 100),
		workerCount: workerCount,
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (r *CourseRobber) Start() {
	for i := 0; i < r.workerCount; i++ {
		r.wg.Add(1)
		go r.worker(i)
	}
}

// Stop 优雅停止抢课管理器
//
// Bug Fix（原代码存在两个问题）：
//
//  1. 顺序问题（导致 panic）：
//     原代码先 cancel() 再 close(taskQueue)，而 worker 的重试逻辑（SubmitTask）
//     会在 ctx.Done() 触发后、goroutine 真正退出前短暂继续执行，此时若向已关闭
//     的 taskQueue 写入，会直接 panic: send on closed channel。
//
//     修复方案：
//     - 先 cancel() 通知所有 worker 停止接收新任务
//     - 再 wg.Wait() 等待所有 worker goroutine 完全退出
//     - 最后用 sync.Once 保证只 close 一次 taskQueue，彻底消除 panic 风险
//
//  2. 重复调用问题：
//     原代码多次调用 Stop() 会多次 close(taskQueue)，导致 panic。
//     修复方案：用 sync.Once 包装整个停止逻辑。
func (r *CourseRobber) Stop() {
	r.stopOnce.Do(func() {
		// 第一步：广播取消信号，通知所有 worker 停止接收新任务
		r.cancel()

		// 第二步：等待所有 worker goroutine 完全退出
		// 必须在 close(taskQueue) 之前完成，确保没有 goroutine 再向 taskQueue 写入
		r.wg.Wait()

		// 第三步：安全关闭 taskQueue（此时所有 worker 已退出，不会再写入）
		close(r.taskQueue)
	})
}

func (r *CourseRobber) SubmitTask(task *RobTask) {
	select {
	case r.taskQueue <- task:
		fmt.Printf("任务已提交: 课程%s, 用户%s\n", task.CourseID, task.UserID)
	case <-r.ctx.Done():
		fmt.Println("任务管理器已停止，无法提交新任务")
	}
}

func (r *CourseRobber) worker(id int) {
	defer r.wg.Done()

	fmt.Printf("工作线程 %d 启动\n", id)

	for {
		select {
		case task, ok := <-r.taskQueue:
			// Bug Fix: 检查 channel 是否已关闭（ok == false 时退出）
			if !ok || task == nil {
				fmt.Printf("工作线程 %d 停止（channel关闭）\n", id)
				return
			}

			// 获取并发许可
			if !r.concurrency.TryAcquire() {
				fmt.Printf("工作线程 %d: 并发数已达上限，等待...\n", id)
				r.concurrency.Acquire()
			}

			// 限流等待
			if err := r.limiter.Wait(r.ctx); err != nil {
				r.concurrency.Release()
				fmt.Printf("工作线程 %d: 限流等待错误: %v\n", id, err)
				continue
			}

			// 执行任务
			result := r.executeTask(task, id)

			// 释放并发许可
			r.concurrency.Release()

			// 处理结果
			r.handleResult(result)

			// 重试逻辑：只在 ctx 未取消时重试，防止 Stop 后继续提交
			if !result.Success && task.RetryCount < task.MaxRetries {
				select {
				case <-r.ctx.Done():
					// ctx 已取消，不再重试
					fmt.Printf("任务取消，不再重试: 课程%s\n", task.CourseID)
				default:
					task.RetryCount++
					fmt.Printf("任务重试 %d/%d: 课程%s\n",
						task.RetryCount, task.MaxRetries, task.CourseID)
					r.SubmitTask(task)
				}
			}

		case <-r.ctx.Done():
			fmt.Printf("工作线程 %d 停止\n", id)
			return
		}
	}
}

func (r *CourseRobber) executeTask(task *RobTask, workerID int) *RobResult {
	start := time.Now()

	fmt.Printf("工作线程 %d 开始执行任务: 课程%s, 用户%s\n",
		workerID, task.CourseID, task.UserID)

	// 模拟网络请求
	time.Sleep(time.Duration(100+workerID*10) * time.Millisecond)

	// 模拟成功/失败（实际应根据业务逻辑判断）
	success := (time.Now().UnixNano() % 5) != 0
	var err error
	if !success {
		err = fmt.Errorf("抢课失败: 课程%s已满或系统错误", task.CourseID)
	}

	duration := time.Since(start)

	return &RobResult{
		Task:     task,
		Success:  success,
		Error:    err,
		Duration: duration,
	}
}

func (r *CourseRobber) handleResult(result *RobResult) {
	if result.Success {
		fmt.Printf("任务成功: 课程%s, 用户%s, 耗时: %v\n",
			result.Task.CourseID, result.Task.UserID, result.Duration)
	} else {
		fmt.Printf("任务失败: 课程%s, 用户%s, 错误: %v\n",
			result.Task.CourseID, result.Task.UserID, result.Error)
	}
}

func main() {
	// 创建抢课管理器：每秒5个请求，突发10个，最大并发3，工作线程5个
	robber := NewCourseRobber(5, 10, 3, 5)

	// 启动工作线程
	robber.Start()

	// 模拟提交任务
	tasks := []*RobTask{
		{CourseID: "CS101", UserID: "20210001", Priority: 1, MaxRetries: 3},
		{CourseID: "MATH201", UserID: "20210002", Priority: 2, MaxRetries: 3},
		{CourseID: "PHYS301", UserID: "20210003", Priority: 1, MaxRetries: 3},
		{CourseID: "CHEM401", UserID: "20210004", Priority: 3, MaxRetries: 3},
		{CourseID: "BIO501", UserID: "20210005", Priority: 2, MaxRetries: 3},
		{CourseID: "ENG601", UserID: "20210006", Priority: 1, MaxRetries: 3},
	}

	for _, task := range tasks {
		robber.SubmitTask(task)
	}

	// 等待一段时间让任务执行
	time.Sleep(2 * time.Second)

	// 优雅停止（可安全多次调用，sync.Once 保证只执行一次）
	robber.Stop()

	fmt.Println("所有任务处理完成")
}
