## 🔧 关于本项目

计划将 `ylibs` 作为一个轻量级、高质量的 **Go 常用组件封装集合**，后续将逐步加入对以下内容的支持：

- ✅ Redis 封装（连接池、分布式锁、缓存工具）
- ✅ MySQL 封装（事务模板、分页器、DSN 管理）
- ✅ 配置加载、文件操作、HTTP 封装等
- ✅ 常用中间件和开发工具集成

**持续更新，欢迎 Star & Issue。**

> 目标是：简洁实用、生产可用，助力开发提效 🚀
# logger

🚀 一个基于 [uber/zap](https://github.com/uber-go/zap) 的通用 Go 日志库，支持 traceId 自动传递、日志切割、多实例、链式调用，适用于各种类型 Go 项目。

---

## ✨ 特性

- ✅ 支持多实例管理（按模块命名隔离）
- ✅ 支持 stdout / 文件日志 / lumberjack 文件切割
- ✅ 支持 traceId 上下文传递，适配分布式链路追踪
- ✅ 标准库 log.Println 自动接管进 zap
- ✅ 支持链式调用：WithContextTrace().Info(...)
- ✅ 高性能、高可读性、线程安全
- ✅ 支持运行时动态调整日志等级

---

## 📦 安装

```bash
go get github.com/syoyyo/ylibs/logger
```

---

## 🧱 初始化配置示例

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/syoyyo/ylibs/logger"
)

func main() {
	cfg := []*logger.LoggerCfg{
		{
			WriterFilePath: []string{"stdout"},
			Level:          logger.DebugLevel,
			Maxsize:        10,
			Maxbackups:     3,
			Maxage:         7,
			Compress:       false,
		},
		{
			WriterFilePath: []string{"log/app.log"},
			Level:          logger.InfoLevel,
			Maxsize:        10,
			Maxbackups:     3,
			Maxage:         7,
			Compress:       true,
		},
	}

	log, err := logger.InitLogger("trace-demo", cfg, 1)
	if err != nil {
		panic(err)
	}

	log.Info("启动服务", "port", 8080)
	log.Debug("调试日志")

	traceLog := log.WithTrace("trace-id-123")
	traceLog.Info("这是一条 traceId 日志")

	ctx := context.WithValue(context.Background(), logger.TraceIDKey, "ctx-trace-id-456")
	ctxLog := log.WithContextTrace(ctx)
	ctxLog.Error("带 context 的错误日志", "code", 500)

	defaultCtxLog := log.WithContextTrace(context.Background())
	defaultCtxLog.Warning("默认 context 自动生成 traceId")

	log2 := logger.GetLogger("default")
	log2.Info("默认 日志器")
	log2.Close()

	fmt.Println("🟡 设置为 Error 等级")
	log.SetAllLevel(logger.ErrorLevel)
	log.Debug("不会打印")
	log.Error("只会打印 Error")

	time.Sleep(500 * time.Millisecond)
	fmt.Println("🟢 恢复 Debug")
	log.SetAllLevel(logger.DebugLevel)
	log.Debug("恢复打印")

	log.SetLevel(0, logger.WarnLevel)
	log.Info("stdout 不会打印")
	log.Warning("stdout 会打印")

	yamlLog, _ := logger.InitLoggerFromFile("yaml", "./log.yaml", 1)
	yamlLog.Info("YAML 配置加载")

	jsonLog, _ := logger.InitLoggerFromFile("json", "./log.json", 1)
	jsonLog.Info("JSON 配置加载")

	log.Close()
}
```

---

## 📄 log.yaml 配置示例

```yaml
- output:
    - stdout
    - ./log/info-yaml.log
  level: info
  maxsize: 20
  maxbackups: 3
  maxage: 5
  compress: false

- output:
    - ./log/only-error-yaml.log
  level: error
  maxsize: 10
  maxbackups: 2
  maxage: 3
  compress: true
```

---

## 📄 log.json 配置示例

```json
[
  {
    "output": [
      "stdout",
      "./log/info-json.log"
    ],
    "level": "info",
    "maxsize": 20,
    "maxbackups": 3,
    "maxage": 5,
    "compress": false
  },
  {
    "output": [
      "./log/only-error-json.log"
    ],
    "level": "error",
    "maxsize": 10,
    "maxbackups": 2,
    "maxage": 3,
    "compress": true
  }
]
```

---

## 🔄 traceId 自动传递

```go
ctx := context.WithValue(context.Background(), logger.TraceIDKey, "trace-abc-123")
log.WithContextTrace(ctx).Info("用户登录成功", "userId", 42)

log.WithTrace("trace-abc-123").Info("用户注册成功", "userId", 42)
你也可以将log放到content中传递到其他地方打印

```

---

## 🌐 Gin 中间件支持

```go
func TraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceId := c.GetHeader("X-Trace-Id")
		if traceId == "" {
			traceId = uuid.NewString()
		}
		ctx := context.WithValue(c.Request.Context(), logger.TraceIDKey, traceId)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
```

---

## 📎 License

MIT License © 2025 Yoyyo

