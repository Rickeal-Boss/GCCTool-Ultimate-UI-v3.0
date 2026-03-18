// Package behavior - 行为模拟模块
//
// 模拟真实用户的操作行为，防止被教务系统识别为自动化工具
package behavior

import (
	"math"
	"math/rand"
	"time"
)

// Point 表示屏幕上的一个点
type Point struct {
	X int
	Y int
}

// MouseTrajectory 鼠标轨迹
type MouseTrajectory struct {
	Points   []Point
	Duration time.Duration
}

// GenerateHumanMouseTrajectory 生成人类鼠标轨迹
//
// 使用贝塞尔曲线生成平滑的鼠标轨迹，添加随机抖动
// 模拟人类不完美的操作（不是完美的直线）
func GenerateHumanMouseTrajectory(start, end Point) MouseTrajectory {
	// 计算距离
	distance := calculateDistance(start, end)

	// 根据距离确定轨迹点数量（每 10 像素一个点）
	pointCount := int(distance / 10)
	if pointCount < 3 {
		pointCount = 3
	}

	// 生成控制点（添加随机偏移，模拟不完美的操作）
	controlPoint1 := Point{
		X: start.X + rand.Intn(int(distance)/3),
		Y: start.Y + rand.Intn(int(distance)/3),
	}
	controlPoint2 := Point{
		X: end.X - rand.Intn(int(distance)/3),
		Y: end.Y - rand.Intn(int(distance)/3),
	}

	// 生成轨迹点
	points := make([]Point, pointCount)
	for i := 0; i < pointCount; i++ {
		t := float64(i) / float64(pointCount-1)
		point := calculateBezierPoint(start, controlPoint1, controlPoint2, end, t)

		// 添加随机抖动（模拟人类不稳定的操作）
		jitter := 5
		point.X += rand.Intn(jitter*2) - jitter
		point.Y += rand.Intn(jitter*2) - jitter

		points[i] = point
	}

	// 计算持续时间（人类鼠标移动速度：300~800 像素/秒）
	speed := 300 + rand.Intn(500) // 300~800 像素/秒
	duration := time.Duration(int64(float64(distance)/float64(speed)*1000)) * time.Millisecond

	return MouseTrajectory{
		Points:   points,
		Duration: duration,
	}
}

// calculateDistance 计算两点之间的距离
func calculateDistance(a, b Point) float64 {
	dx := float64(b.X - a.X)
	dy := float64(b.Y - a.Y)
	return math.Sqrt(dx*dx + dy*dy)
}

// calculateBezierPoint 计算贝塞尔曲线上的点
//
// 三次贝塞尔曲线公式：
// B(t) = (1-t)³*P0 + 3*(1-t)²*t*P1 + 3*(1-t)*t²*P2 + t³*P3
func calculateBezierPoint(p0, p1, p2, p3 Point, t float64) Point {
	return Point{
		X: int(math.Pow(1-t, 3)*float64(p0.X) +
			3*math.Pow(1-t, 2)*t*float64(p1.X) +
			3*(1-t)*math.Pow(t, 2)*float64(p2.X) +
			math.Pow(t, 3)*float64(p3.X)),
		Y: int(math.Pow(1-t, 3)*float64(p0.Y) +
			3*math.Pow(1-t, 2)*t*float64(p1.Y) +
			3*(1-t)*math.Pow(t, 2)*float64(p2.Y) +
			math.Pow(t, 3)*float64(p3.Y)),
	}
}

// SimulateHumanClick 模拟人类点击
//
// 在按钮中心附近随机偏移的位置点击
// 模拟人类不精确的点击操作
func SimulateHumanClick(buttonCenter Point) Point {
	// 随机偏移范围：按钮中心 ±20 像素
	offset := 20
	return Point{
		X: buttonCenter.X + rand.Intn(offset*2) - offset,
		Y: buttonCenter.Y + rand.Intn(offset*2) - offset,
	}
}

// SimulateHumanDelay 模拟人类操作延迟
//
// 模拟人类操作之间的随机延迟
// 返回延迟时间：100~500ms
func SimulateHumanDelay() time.Duration {
	return time.Duration(100+rand.Intn(400)) * time.Millisecond
}

// SimulateTypingDelay 模拟人类输入延迟
//
// 模拟人类输入每个字符之间的延迟
// 返回延迟时间：50~200ms
func SimulateTypingDelay() time.Duration {
	return time.Duration(50+rand.Intn(150)) * time.Millisecond
}

// SimulateReadingDelay 模拟人类阅读延迟
//
// 模拟人类阅读文本时的延迟
// 返回延迟时间：500~2000ms
func SimulateReadingDelay(textLength int) time.Duration {
	// 阅读速度：200~500 字/分钟
	readingSpeed := 200 + rand.Intn(300) // 200~500 字/分钟
	readingTime := float64(textLength) / float64(readingSpeed) * 60 * 1000 // 转换为毫秒

	// 添加随机抖动
	jitter := readingTime * 0.2 // ±20%
	readingTime += (rand.Float64()*2 - 1) * jitter

	return time.Duration(readingTime) * time.Millisecond
}

// SimulateThinkingDelay 模拟人类思考延迟
//
// 模拟人类决策时的思考延迟
// 返回延迟时间：500~3000ms
func SimulateThinkingDelay() time.Duration {
	return time.Duration(500+rand.Intn(2500)) * time.Millisecond
}
