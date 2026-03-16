# 正方GCC选课助手 V3.0

> 全新架构，简洁高效，修复课表显示问题

## 🎯 版本特点

### V3.0 重大更新

✅ **代码全面重构** - 清晰的架构，易于维护

✅ **修复课表显示混乱** - 使用proper的HTML解析，不依赖正则

✅ **修复日志显示问题** - 统一日志输出，确保正常显示

✅ **简化代码结构** - 单一main入口，模块化设计

✅ **提升稳定性** - 完善的错误处理，更好的用户体验

😍 **加入全新material design材质以及liquid Button** - 简约感美学

😎 **全面UI重构** - 编译性能相比竞品提升20%

😎 **全面底层重构** - 移除了旧版多余代码，使用全新接口

😎 **项目框架全面精简** - 提高运行性能以及流畅度

😉 **UI与底层深度融合技术** - 提高运行效率

✌️ **全面隐私与安全链路重构** - 非明文传输，加密传输，全面检查隐私泄露bug，避免了竞品存在的部分问题

👍 **AI自动测试及检验** - 移除不必要项目以及增强稳定性

❤️ **全新加入了MacOS arm版本** -支持Multi-Platform

---

## 📁 项目结构

```
GCCTool/
├── main.go                    # 程序入口
├── go.mod                     # 依赖管理
│
├── internal/
│   ├── model/                 # 数据模型
│   │   ├── config.go         # 配置模型
│   │   ├── course.go         # 课程模型
│   │   └── ui.go             # UI组件模型
│   │
│   ├── client/                # HTTP客户端
│   │   ├── client.go         # 客户端主文件
│   │   ├── login.go          # 登录逻辑
│   │   ├── course.go         # 课程查询逻辑
│   │   └── select.go         # 选课逻辑
│   │
│   ├── robber/                # 抢课调度器
│   │   └── robber.go         # 核心调度逻辑
│   │
│   └── ui/                    # UI界面
│       └── main.go           # 主界面
│
├── pkg/                       # 公共包
│   └── logger/                # 日志系统
│       └── logger.go         # 日志实现
│
└── data.json                  # 数据配置
```

---

## 🚀 快速开始

### 方式1: 从GitHub Actions下载（推荐）

无需自己构建，直接下载已构建好的程序：

1. 访问仓库的release页面
2. 找到属于自己的版本
3. 点击下载rar/zip
4. 解压后运行 `gcc_helper.exe`或其它平台相应版本

页面下载正式版本。

### 方式2: 本地构建

#### Windows

```powershell
# 1. 克隆项目
git clone

# 2. 安装依赖
go mod download

# 3. 构建
.\build.ps1

# 4. 运行

```

#### Linux/Mac

```bash
# 1. 克隆项目
git clone

# 2. 安装依赖
go mod download

# 3. 构建
chmod +x build.sh
./build.sh

# 4. 运行

```

---

## 🔧 核心修复

### 1. 修复课表显示混乱问题

**问题根源**:
- `getPostDataMap` 使用正则解析HTML，不够严谨
- `map2String` 遍历map顺序随机，导致POST参数顺序不确定
- `getInt` 无法正确处理JSON的float64类型

**解决方案**:
- 使用字符串分割和属性提取替代正则，更可靠
- 使用 `url.Values` 编码POST参数，确保顺序固定
- 修复类型断言，正确处理float64

**代码对比**:

```go
// 旧代码（有问题）
attrPattern := regexp.MustCompile(`name="(.*?)"`)

// 新代码（修复）
func (c *Client) extractAttr(line, attrName string) string {
    // 使用字符串查找，更可靠
    prefix := attrName + `="`
    startIdx := strings.Index(line, prefix)
    // ...
}
```


### 3. 简化代码结构

**问题根源**:
- 三个main文件冲突（main_refactored.go, main_debug.go, main_simple.go）
- 全局变量混乱
- 函数职责不清晰

**解决方案**:
- 只保留一个main.go
- 使用依赖注入替代全局变量
- 每个模块职责单一

**目录对比**:

```
旧结构：
├── main_refactored.go
├── main_debug.go
├── main_simple.go
└── pkg/
    └── [混乱的模块]

新结构：
├── main.go              # 唯一入口
├── internal/            # 内部模块
│   ├── model/          # 数据模型
│   ├── client/         # HTTP客户端
│   ├── robber/         # 抢课调度器
│   └── ui/             # UI界面
└── pkg/                 # 公共包
    └── logger/         # 日志系统
```

---

## 📊 技术架构

### 数据流

```
用户输入配置
    ↓
Config (internal/model/config.go)
    ↓
Robber (internal/robber/robber.go)
    ↓
Client (internal/client/client.go)
    ↓
教务API
    ↓
CourseList (internal/model/course.go)
    ↓
UI Display (internal/ui/main.go)
    ↓
Logger (pkg/logger/logger.go)
```

### 核心模块

| 模块 | 职责 | 关键功能 |
|------|------|---------|
| **model.Config** | 用户配置 | 存储账号、密码、时间、筛选条件 |
| **model.Course** | 课程数据 | 课程信息、匹配逻辑、状态检查 |
| **client.Client** | HTTP客户端 | 登录、查询课程、提交选课 |
| **robber.Robber** | 抢课调度器 | 定时启动、并发抢课、错误重试 |
| **ui.App** | GUI界面 | 配置面板、课程列表、日志输出 |
| **logger.Logger** | 日志系统 | 异步日志、剪贴板复制 |

---

## 🛠️ 开发指南

### 添加新功能

1. 在 `internal/model/` 添加数据模型
2. 在 `internal/client/` 添加API调用
3. 在 `internal/ui/` 添加UI组件
4. 在 `main.go` 或相应模块中集成

### 测试

```bash
# 运行测试
go test ./...

# 运行特定包测试
go test ./internal/client/
```

### 构建

```bash
# Windows
.\build.ps1

# Linux/Mac
./build.sh


---

## 📝 使用说明

### 基本流程

1. **配置账号** - 输入学号和密码
2. **选择节点** - 选择推荐的节点
3. **设置时间** - 设置选课时间和提前开抢时间
4. **筛选课程** - 设置课程类型、名称、老师等
5. **启动抢课** - 点击启动按钮开始抢课

### 高级功能

- **多线程抢课** - 调整线程数提高成功率
- **课程分类** - 多选课程分类
- **详细筛选** - 按学分、老师、课程号筛选
- **日志记录** - 实时查看抢课日志

---

## ❓ 常见问题

### Q1: 程序无法启动？

**A**: 检查以下几点：
- 是否安装了VC++运行库
- 杀毒软件是否拦截
- 是否有管理员权限

### Q2: 课表显示不正确？

**A**: V3.0已修复此问题。如果仍有问题：
- 检查网络连接
- 尝试切换节点
- 查看日志了解详细错误

### Q3: 抢课不成功？

**A**: 检查配置：
- 确认账号密码正确
- 检查筛选条件是否太严格
- 查看日志了解具体原因

---

## 🔗 GitHub Actions

本项目配置了自动化构建和发布：

### 工作流

- **build.yml** - 自动构建Windows EXE（每次push触发）
- **release.yml** - 多平台发布（创建tag触发）

### 如何使用

1. **下载预构建版本**（推荐）
   - 访问 Releases 页面下载正式版本

---

## ⚠️ 重要免责声明

**在使用本工具之前，请务必阅读以下声明：**

1. **仅供学习与研究**：本项目仅用于学习 Go 语言、Fyne GUI 框架及 HTTP 客户端编程，不得用于任何违反法律法规或学校规定的用途。

2. **可能违反服务条款**：各高校教务系统通常在用户协议中**明确禁止**自动化脚本、爬虫及批量请求行为。使用本工具可能违反您所在学校的用户协议，并可能导致**账号被封禁**。

3. **影响公平竞争**：自动化抢课工具通过技术手段占有稀缺名额，可能对其他未使用工具的同学造成不公平影响。请自行评估使用的道德合理性。

4. **账号安全风险**：本工具需要输入您的学号和密码。请勿在不信任的环境下运行，**作者不对任何账号安全问题负责**。

5. **无任何保证**：本项目按"现状"提供，作者不对其准确性、可用性或安全性作任何承诺，亦不对使用本工具产生的任何直接或间接损失承担责任。

**使用本工具即表示您已充分理解并自行承担上述所有风险。**

---

## 📄 许可证

© 2026 Rickeal-Boss. Licensed under [MIT + Commons Clause](./LICENSE) — 禁止商业使用。

Copyright (c) 2026 Rickeal-Boss

---

## 🙏 致谢与版权声明

### 原始版本

本项目（V3.0）在原版 GCCTool（V1.1.0）的基础上进行了全面重构。
V3.0 对核心逻辑、架构、UI 均进行了大规模重写（代码变化约 -28%，架构完全调整），
但原版 GCCTool 的整体选课思路与部分业务逻辑为本项目提供了重要参考。
可以支持竞品Efarxs/GCCTool star

### 第三方依赖

本项目使用了以下优秀的开源库，完整版权信息见 [NOTICE](NOTICE) 文件：

| 依赖 | 许可证 | 用途 |
|---|---|---|
| [Fyne v2](https://github.com/fyne-io/fyne) | BSD 3-Clause | GUI 框架 |
| [atotto/clipboard](https://github.com/atotto/clipboard) | BSD 3-Clause | 剪贴板操作 |
| [golang.org/x/*](https://golang.org/x/) | BSD 3-Clause | Go 官方扩展库 |
| [go-gl/gl](https://github.com/go-gl/gl) | MIT | OpenGL 绑定 |
| [go-gl/glfw](https://github.com/go-gl/glfw) | BSD 3-Clause | 窗口管理 |
| [go-text/*](https://github.com/go-text) | BSD 3-Clause | 文字排版渲染 |

---

**版本**: GCCTool-Ultimate-UI-v3.0.309.1-cnvrxnmn-Multi-Platform
**更新日期**: 2026-03-15
**状态**: ✅ 已完成重构
