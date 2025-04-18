package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/syoyyo/ylibs/logger"
)

func TestCfg(t *testing.T) {
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

	log.Close()
	yamlLog, err := logger.InitLoggerFromFile("yaml", "../log.yaml", 1)
	yamlLog.Error("Error", "port", 8080, "env", "dev", "err", err)
	yamlLog.Info("Info", "port", 8080, "env", "dev", "err", err)
	// 或 JSON
	jsonLog, err := logger.InitLoggerFromFile("json", "../log.json", 1)
	jsonLog.Error("Error", "port", 8080, "env", "dev", "err", err)
	jsonLog.Info("Info", "port", 8080, "env", "dev", "err", err)

}
