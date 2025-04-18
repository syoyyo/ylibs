// Package logger 提供一个基于 zap 封装的日志系统，支持多输出、动态级别调整、
// TraceID 注入、标准日志重定向等特性。
//
// 特性：
//   - 支持多输出目标（stdout, stderr, 文件等）
//   - 支持 JSON/YAML 配置
//   - 支持 TraceID 上下文注入
//   - 支持动态修改日志级别
//   - 支持标准日志重定向（zap.RedirectStdLog）
//
// 示例：
//
//	cfgs, _ := logger.GetLoggerCfgByYaml([]byte(`
//	- output: ["stdout"]
//	  level: "debug"
//	  maxsize: 10
//	  maxbackups: 5
//	  maxage: 30
//	  compress: true
//	`))
//	log, _ := logger.InitLogger("main", cfgs, 1)
//	log.Info("Hello world")

package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"
	zap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v3"
)

// WarpLog 是日志接口，封装 zap 的功能，支持链式调用和 Trace 注入。
type WarpLog interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Error(msg string, fields ...any)
	Warning(msg string, fields ...any)
	Fatal(msg string, fields ...any)
	Panic(msg string, fields ...any)
	SetAllLevel(level LogLevel)
	SetLevel(idx int, level LogLevel)
	WithContextTrace(ctx context.Context) TraceLog
	WithTrace(traceId string) TraceLog
	RedirectStdLog()
	Close()
}

// TraceLog 提供基于 context 的日志接口，自动注入 traceId。
type TraceLog interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Error(msg string, fields ...any)
	Warning(msg string, fields ...any)
	Fatal(msg string, fields ...any)
	Panic(msg string, fields ...any)
	WithContext(ctx context.Context) TraceLog
}

// LogLevel 表示日志等级，如 debug/info/warn/error。
type LogLevel string

func (l *LogLevel) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	t, err := parseLogLevel(s)
	if err != nil {
		return err
	}
	*l = LogLevel(t)
	return nil
}

func (l *LogLevel) UnmarshalYAML(node *yaml.Node) error {
	t, err := parseLogLevel(node.Value)
	if err != nil {
		return err
	}
	*l = LogLevel(t)
	return nil
}
func parseLogLevel(s string) (string, error) {
	switch v := strings.ToLower(strings.TrimSpace(s)); v {
	case "debug", "info", "warn", "warning", "error":
		if v == "warning" {
			return "warn", nil
		}
		return v, nil
	default:
		return "", fmt.Errorf("invalid log level: %s", s)
	}
}

func (l LogLevel) LogLevel() zapcore.Level {
	level, err := zapcore.ParseLevel(string(l))
	if err != nil {
		return zapcore.InfoLevel
	}
	return level
}

const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
)

type contextKey string

const TraceIDKey contextKey = "TraceLogId"

// LoggerCfg 定义日志的配置项，支持输出路径、文件大小、压缩等。
type LoggerCfg struct {
	WriterFilePath []string `json:"output" yaml:"output"`         // 输出路径
	Maxsize        int      `json:"maxsize" yaml:"maxsize"`       // 单个文件最大 MB
	Maxbackups     int      `json:"maxbackups" yaml:"maxbackups"` // 最大备份文件数
	Maxage         int      `json:"maxage" yaml:"maxage"`         // 最大保留天数
	Compress       bool     `json:"compress" yaml:"compress"`     // 是否压缩
	Level          LogLevel `json:"level" yaml:"level"`           // 日志等级
}

var (
	loggerMap  = make(map[string]*logger)
	loggerLock sync.RWMutex
)

// GetLoggerCfgByJson 从 JSON 字节读取配置。
func GetLoggerCfgByJson(data []byte) ([]*LoggerCfg, error) {
	var cfgs []*LoggerCfg
	if err := json.Unmarshal(data, &cfgs); err != nil {
		var cfg = &LoggerCfg{}
		err = json.Unmarshal(data, cfg)
		if err == nil {
			return []*LoggerCfg{cfg}, nil
		}
		return nil, err
	}
	return cfgs, nil
}

// GetLoggerCfgByYaml 从 YAML 字节读取配置。
func GetLoggerCfgByYaml(data []byte) ([]*LoggerCfg, error) {
	var cfgs []*LoggerCfg
	if err := yaml.Unmarshal(data, &cfgs); err != nil {
		var cfg = &LoggerCfg{}
		err = yaml.Unmarshal(data, cfg)
		if err == nil {
			return []*LoggerCfg{cfg}, nil
		}
		return nil, err
	}
	return cfgs, nil

}

func loadLoggerCfgFromFile(path string) ([]*LoggerCfg, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	switch {
	case path == "": // fallback
		return nil, fmt.Errorf("config path is empty")
	case path[len(path)-5:] == ".json":
		return GetLoggerCfgByJson(data)
	case path[len(path)-5:] == ".yaml" || path[len(path)-4:] == ".yml":
		return GetLoggerCfgByYaml(data)
	default:
		return nil, fmt.Errorf("unsupported config file: %s", path)
	}
}

// InitLogger 使用指定配置初始化一个新的日志实例。
func InitLogger(tag string, cfg []*LoggerCfg, skipCall int) (WarpLog, error) {
	return generate(tag, cfg, skipCall)
}

// InitLoggerFromFile 从配置文件初始化日志（支持 json/yaml）。
func InitLoggerFromFile(tag string, path string, skipCall int) (WarpLog, error) {
	cfgs, err := loadLoggerCfgFromFile(path)
	if err != nil {
		return nil, err
	}
	return InitLogger(tag, cfgs, skipCall)
}

// GetLogger 获取已存在的日志实例，若不存在则返回默认 stdout 日志。
func GetLogger(tag string) WarpLog {
	loggerLock.RLock()
	inst := loggerMap[tag]
	loggerLock.RUnlock()
	if inst == nil {
		atomicLevel := zap.NewAtomicLevelAt(InfoLevel.LogLevel())
		core := zapcore.NewCore(getEncoder(), zapcore.AddSync(os.Stdout), atomicLevel)
		zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
		inst = &logger{logger: zapLogger.Sugar(), atomicLevels: []*zap.AtomicLevel{&atomicLevel}}
		loggerLock.Lock()
		loggerMap[tag] = inst
		loggerLock.Unlock()
	}
	return inst
}

// generate returns or creates a named logger with config
func generate(tag string, cfg []*LoggerCfg, skipCall int) (WarpLog, error) {
	if len(cfg) == 0 {
		return nil, fmt.Errorf("logger config is empty")
	}

	loggerLock.RLock()
	inst := loggerMap[tag]
	loggerLock.RUnlock()
	if inst != nil {
		return inst, nil
	}

	cores := make([]zapcore.Core, 0)
	encoder := getEncoder()
	var atomicLevels []*zap.AtomicLevel
	for _, v := range cfg {
		if v == nil {
			return nil, fmt.Errorf("logger config contains nil item")
		}
		writer := getWriter(v)
		atomicLevel := zap.NewAtomicLevelAt(v.Level.LogLevel()) // 支持后续变更
		core := zapcore.NewCore(encoder, writer, atomicLevel)
		cores = append(cores, core)
		atomicLevels = append(atomicLevels, &atomicLevel)
	}

	zapLogger := zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddCallerSkip(skipCall))
	sugar := zapLogger.Sugar()
	inst = &logger{
		logger:       sugar,
		atomicLevels: atomicLevels,
	}
	loggerLock.Lock()
	loggerMap[tag] = inst
	loggerLock.Unlock()

	return inst, nil
}

func getEncoder() zapcore.Encoder {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	cfg.CallerKey = "caller"
	cfg.TimeKey = "time"
	return zapcore.NewConsoleEncoder(cfg)
}

func getWriter(cfg *LoggerCfg) zapcore.WriteSyncer {
	var writers []zapcore.WriteSyncer

	for _, path := range cfg.WriterFilePath {
		path = strings.TrimSpace(path)
		if strings.EqualFold(path, "stdout") {
			writers = append(writers, zapcore.AddSync(os.Stdout))
		} else if strings.EqualFold(path, "stderr") {
			writers = append(writers, zapcore.AddSync(os.Stderr))
		} else {
			writers = append(writers, zapcore.AddSync(&lumberjack.Logger{
				Filename:   path,
				MaxSize:    cfg.Maxsize,
				MaxBackups: cfg.Maxbackups,
				MaxAge:     cfg.Maxage,
				Compress:   cfg.Compress,
			}))
		}
	}

	return zapcore.NewMultiWriteSyncer(writers...)
}

type logger struct {
	logger       *zap.SugaredLogger
	atomicLevels []*zap.AtomicLevel
	WarpLog
}

// key:TraceIDKey
func (l *logger) WithContextTrace(ctx context.Context) TraceLog {
	if traceId := ctx.Value(TraceIDKey); traceId == nil {
		ctx = context.WithValue(ctx, TraceIDKey, uuid.NewString())
	}
	return &traceLogger{
		base: l,
		ctx:  ctx,
	}
}
func (l *logger) WithTrace(traceId string) TraceLog {
	ctx := context.Background()
	ctx = context.WithValue(ctx, TraceIDKey, traceId)
	return &traceLogger{
		base: l,
		ctx:  ctx,
	}
}
func (l *logger) RedirectStdLog() {
	if l.logger != nil {
		_ = zap.RedirectStdLog(l.logger.Desugar())
	}
}

func (l *logger) Debug(msg string, fields ...any) {
	l.logger.Debugw(msg, fields...)
}

func (l *logger) Info(msg string, fields ...any) {
	l.logger.Infow(msg, fields...)
}

func (l *logger) Warning(msg string, fields ...any) {
	l.logger.Warnw(msg, fields...)
}

func (l *logger) Error(msg string, fields ...any) {
	l.logger.Errorw(msg, fields...)
}

func (l *logger) Fatal(msg string, fields ...any) {
	l.logger.Fatalw(msg, fields...)
}

func (l *logger) Panic(msg string, fields ...any) {
	l.logger.Panicw(msg, fields...)
}
func (l *logger) SetAllLevel(level LogLevel) {
	newZapLevel := level.LogLevel()
	for _, lvl := range l.atomicLevels {
		lvl.SetLevel(newZapLevel)
	}
}

// idx start with 0
func (l *logger) SetLevel(idx int, level LogLevel) {
	if len(l.atomicLevels) > idx {
		l.atomicLevels[idx].SetLevel(level.LogLevel())
	}
}
func (l *logger) Close() {
	if l.logger != nil {
		_ = l.logger.Sync()
	}
}

type traceLogger struct {
	base *logger
	ctx  context.Context
}

func (c *traceLogger) Info(msg string, fields ...any) {
	fields = c.injectTraceId(fields)
	c.base.logger.Infow(msg, fields...)
}

func (c *traceLogger) Error(msg string, fields ...any) {
	fields = c.injectTraceId(fields)
	c.base.logger.Errorw(msg, fields...)
}

func (c *traceLogger) Debug(msg string, fields ...any) {
	fields = c.injectTraceId(fields)
	c.base.logger.Debugw(msg, fields...)
}

func (c *traceLogger) Warning(msg string, fields ...any) {
	fields = c.injectTraceId(fields)
	c.base.logger.Warnw(msg, fields...)
}

func (c *traceLogger) Fatal(msg string, fields ...any) {
	fields = c.injectTraceId(fields)
	c.base.logger.Fatalw(msg, fields...)
}

func (c *traceLogger) Panic(msg string, fields ...any) {
	fields = c.injectTraceId(fields)
	c.base.logger.Panicw(msg, fields...)
}

func (c *traceLogger) injectTraceId(fields []any) []any {
	if traceId, ok := c.ctx.Value(TraceIDKey).(string); ok && traceId != "" {
		fields = append(fields, "traceId", traceId)
	}
	return fields
}

// WithTrace injects traceId into the context and return new TraceLog
func (c *traceLogger) WithContext(ctx context.Context) TraceLog {
	return c.base.WithContextTrace(ctx)
}
