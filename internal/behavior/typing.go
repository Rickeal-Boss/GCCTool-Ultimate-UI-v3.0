// Package behavior - 行为模拟模块
//
// 模拟真实用户的操作行为，防止被教务系统识别为自动化工具
package behavior

import (
	"math/rand"
	"strings"
	"time"
)

// SimulateHumanTyping 模拟人类输入
//
// 模拟人类输入文本时的随机延迟和错误
// 参数：
//   - text: 要输入的文本
//   - inputFunc: 输入函数（每次调用输入一个字符）
//   - deleteFunc: 删除函数（每次调用删除一个字符）
//
// 功能：
//   - 每个字符之间有随机延迟（50~200ms）
//   - 5% 概率输入错误字符
//   - 输入错误后立即删除并重新输入
func SimulateHumanTyping(text string, inputFunc func(ch string), deleteFunc func()) {
	for i, ch := range text {
		// 输入字符
		inputFunc(string(ch))

		// 模拟输入延迟
		time.Sleep(SimulateTypingDelay())

		// 模拟输入错误（5% 概率，但最后一个字符不模拟错误）
		if i < len(text)-1 && rand.Float32() < 0.05 {
			// 输入错误字符
			errorChar := string(ch + 1) // 输入下一个字符
			inputFunc(errorChar)
			time.Sleep(SimulateTypingDelay())

			// 删除错误字符
			deleteFunc()
			time.Sleep(SimulateTypingDelay())

			// 重新输入正确字符
			inputFunc(string(ch))
			time.Sleep(SimulateTypingDelay())
		}
	}
}

// SimulateHumanPasswordInput 模拟人类密码输入
//
// 模拟人类输入密码时的行为
// 参数：
//   - password: 要输入的密码
//   - inputFunc: 输入函数（每次调用输入一个字符）
//
// 功能：
//   - 密码输入速度比普通输入快（50~100ms/字符）
//   - 不模拟输入错误（密码错误会被系统检测）
func SimulateHumanPasswordInput(password string, inputFunc func(ch string)) {
	for _, ch := range password {
		// 输入字符
		inputFunc(string(ch))

		// 密码输入速度较快（50~100ms）
		time.Sleep(time.Duration(50+rand.Intn(50)) * time.Millisecond)
	}
}

// SimulateHumanFormFilling 模拟人类填写表单
//
// 模拟人类填写表单时的行为
// 参数：
//   - fields: 表单字段映射（字段名 -> 值）
//   - inputFunc: 输入函数（参数：字段名、值）
//
// 功能：
//   - 按字段顺序填写
//   - 每个字段之间有随机延迟（100~500ms）
//   - 填写完成后有短暂的"检查"延迟（500~1500ms）
func SimulateHumanFormFilling(fields map[string]string, inputFunc func(field, value string)) {
	fieldNames := []string{}
	for name := range fields {
		fieldNames = append(fieldNames, name)
	}

	// 按顺序填写字段
	for i, name := range fieldNames {
		value := fields[name]

		// 模拟阅读字段名称
		time.Sleep(SimulateReadingDelay(len(name)))

		// 填写字段
		inputFunc(name, value)

		// 字段之间的延迟
		if i < len(fieldNames)-1 {
			time.Sleep(SimulateHumanDelay())
		}
	}

	// 填写完成后，模拟检查表单
	time.Sleep(time.Duration(500+rand.Intn(1000)) * time.Millisecond)
}

// SimulateHumanScrolling 模拟人类滚动
//
// 模拟人类滚动页面时的行为
// 参数：
//   - scrollFunc: 滚动函数（参数：滚动距离）
//   - totalDistance: 总滚动距离（像素）
//
// 功能：
//   - 分多次滚动（每次滚动 100~300 像素）
//   - 每次滚动之间有随机延迟（50~200ms）
//   - 模拟不均匀的滚动速度
func SimulateHumanScrolling(scrollFunc func(distance int), totalDistance int) {
	remaining := totalDistance
	for remaining > 0 {
		// 每次滚动 100~300 像素
		distance := 100 + rand.Intn(200)
		if distance > remaining {
			distance = remaining
		}

		// 执行滚动
		scrollFunc(distance)

		// 滚动延迟
		time.Sleep(time.Duration(50+rand.Intn(150)) * time.Millisecond)

		remaining -= distance
	}
}

// SimulateHumanNavigation 模拟人类导航
//
// 模拟人类在页面之间的导航行为
// 参数：
//   - navigateFunc: 导航函数（参数：URL）
//   - urls: 要访问的 URL 列表
//
// 功能：
//   - 按顺序访问 URL
//   - 每个页面停留随机时间（1~3 秒）
//   - 模拟返回上一页（10% 概率）
func SimulateHumanNavigation(navigateFunc func(url string), urls []string) {
	visited := make(map[int]bool)
	for i, url := range urls {
		// 访问页面
		navigateFunc(url)

		// 模拟浏览页面
		time.Sleep(time.Duration(1000+rand.Intn(2000)) * time.Millisecond)

		// 10% 概率返回上一页
		if i > 0 && rand.Float32() < 0.1 {
			if i-1 >= 0 {
				navigateFunc(urls[i-1])
				time.Sleep(time.Duration(500+rand.Intn(1000)) * time.Millisecond)
				// 重新访问当前页
				navigateFunc(url)
				time.Sleep(time.Duration(500+rand.Intn(1000)) * time.Millisecond)
			}
		}
	}
}

// SimulateHumanIdle 模拟人类空闲
//
// 模拟人类在操作之间的空闲时间
// 参数：
//   - idleType: 空闲类型
//     - "short": 短暂空闲（500~1500ms）
//     - "medium": 中等空闲（2~5 秒）
//     - "long": 长时间空闲（5~10 秒）
func SimulateHumanIdle(idleType string) time.Duration {
	switch strings.ToLower(idleType) {
	case "short":
		return time.Duration(500+rand.Intn(1000)) * time.Millisecond
	case "medium":
		return time.Duration(2000+rand.Intn(3000)) * time.Millisecond
	case "long":
		return time.Duration(5000+rand.Intn(5000)) * time.Millisecond
	default:
		return SimulateHumanDelay()
	}
}

// SimulateHumanHesitation 模拟人类犹豫
//
// 模拟人类在做决策时的犹豫行为
// 返回延迟时间：500~3000ms
func SimulateHumanHesitation() time.Duration {
	return time.Duration(500+rand.Intn(2500)) * time.Millisecond
}
