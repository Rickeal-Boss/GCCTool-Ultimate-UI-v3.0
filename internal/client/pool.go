package client

import (
	"time"

	"github.com/Rickeal-Boss/GCCTool-Ultimate-UI-v3.0/internal/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// 选课参数池（页面隐藏参数 + 轮次）+ 90 秒复用
//
// 背景：正方服务端限流阈值很低（实测连续 4~7 个请求即 429）。
// V3.0/V3.1 每轮抢课都重新走"探测 GET + 首页 GET + Display POST + PartDisplay
// POST"共 4 个请求，10 个 Worker × 毫秒级轮询 ≈ 每分钟上百请求，必踩限流。
//
// 修复（实机验证）：成功拉取一次课程列表后，把"首页 + Display"下发的隐藏参数
// 与轮次列表缓存 90s；窗口内后续轮询只发 1 个 PartDisplay 请求
// （生产中 4 请求/90s → 1 请求/90s，远在配额内）。窗口过期自动回落完整链路。
//
// 锁约定：参数池与轮次是同一份会话状态的两种视图，必须同锁同时更新/作废
// （复用 Client.mu，与 selectInitParams 同一把锁）。
// ─────────────────────────────────────────────────────────────────────────────

// paramPoolTTL 参数池复用窗口。
//
// 取 90s 的依据：选课提交必需的服务端下发参数（xkkz_xh 等）在会话期内稳定，
// 90s 内的复用不会用到过期值；同时把请求量压到约 1/90。
const paramPoolTTL = 90 * time.Second

// refreshPool 写入参数池并开启 90s 复用窗口（完整链路成功后调用）
func (c *Client) refreshPool(params map[string]string, rounds []model.Round) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cp := make(map[string]string, len(params))
	for k, v := range params {
		cp[k] = v
	}
	c.selectInitParams = cp
	c.poolRounds = append([]model.Round(nil), rounds...)
	c.poolValidUntil = time.Now().Add(paramPoolTTL)
}

// poolValid 参数池当前是否可复用
func (c *Client) poolValid() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.selectInitParams != nil && time.Now().Before(c.poolValidUntil)
}

// poolSnapshot 取出参数池快照（参数副本 + 轮次副本 + 是否有效）
func (c *Client) poolSnapshot() (map[string]string, []model.Round, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.selectInitParams == nil || !time.Now().Before(c.poolValidUntil) {
		return nil, nil, false
	}
	params := make(map[string]string, len(c.selectInitParams))
	for k, v := range c.selectInitParams {
		params[k] = v
	}
	rounds := append([]model.Round(nil), c.poolRounds...)
	return params, rounds, true
}

// invalidatePool 作废参数池复用窗口（不丢参数值，只让下次走完整链路）
func (c *Client) invalidatePool() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.poolValidUntil = time.Time{}
}

// SetRounds 缓存轮次列表（与参数池同锁；预热路径在解析首页后调用）
func (c *Client) SetRounds(rounds []model.Round) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.poolRounds = append([]model.Round(nil), rounds...)
}

// Rounds 返回轮次列表副本
func (c *Client) Rounds() []model.Round {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]model.Round(nil), c.poolRounds...)
}

// DescribeRounds 返回人类可读的轮次描述（用于日志/告警：教务实际开放了哪些轮次）
func (c *Client) DescribeRounds() []string {
	rounds := c.Rounds()
	out := make([]string, 0, len(rounds))
	for _, r := range rounds {
		name := r.Name
		if name == "" {
			name = "未命名页签"
		}
		out = append(out, name+"(kklxdm="+r.Kklxdm+")")
	}
	return out
}

// roundFor 按课程类型选出应对应的轮次（整组取用，不混用不同轮次的参数）。
//
// 返回 (轮次, 是否发生了类型回落)：
//   - 配置类型的任一候选 kklxdm 命中开放轮次 → 用该轮次，relaxed=false；
//   - 未命中 → 回落首个开放轮次，relaxed=true（调用方必须告警；
//     此时课程类型过滤也应放宽，否则该轮次课程会被全部滤掉 → 死路）；
//   - 无任何轮次信息 → 用参数池里的 firstXxx 合成，relaxed=true。
func (c *Client) roundFor(courseType string) (model.Round, bool) {
	c.mu.Lock()
	rounds := append([]model.Round(nil), c.poolRounds...)
	pool := c.selectInitParams
	c.mu.Unlock()

	code := model.CourseTypeCodeKklxdm(courseType)
	candidates := model.CourseTypeKklxdmCandidates(courseType)

	for _, cand := range candidates {
		for _, r := range rounds {
			if r.Kklxdm == cand {
				return r, false
			}
		}
	}

	if len(rounds) > 0 {
		return rounds[0], true
	}

	// 无轮次信息：用参数池的 firstXxx 合成（单轮次开放时首页就是这么下发的）
	synth := model.Round{
		Kklxdm: code,
		XkkzID: pool["firstXkkzId"],
		NjdmID: pool["firstNjdmId"],
		ZyhID:  pool["firstZyhId"],
		XkkzXh: pool["firstXkkzXh"],
	}
	if synth.XkkzID == "" {
		synth.XkkzID = pool["xkkz_id"]
	}
	if synth.XkkzXh == "" {
		synth.XkkzXh = pool["xkkz_xh"]
	}
	if synth.Kklxdm == "" {
		synth.Kklxdm = pool["firstKklxdm"]
	}
	return synth, true
}
