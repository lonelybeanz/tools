package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var globalLogger *zap.SugaredLogger

// Config 日志配置
type Config struct {
	FilePath    string `yaml:"filePath"`    // 日志文件路径, e.g., /var/log/app.log. 错误日志会自动生成为 /var/log/app.error.log
	MaxSize     int    `yaml:"maxSize"`     // 每个日志文件最大尺寸(MB)
	MaxBackups  int    `yaml:"maxBackups"`  // 保留最近多少个文件
	MaxAge      int    `yaml:"maxAge"`      // 保留最近多少天
	Level       string `yaml:"level"`       // 日志级别: debug, info, warn, error
	Compress    bool   `yaml:"compress"`    // 是否压缩旧文件
	Development bool   `yaml:"development"` // 是否为开发模式
	MsgOnly     bool   `yaml:"msgOnly"`     // 是否只输出 message 字段
}

func init() {
	// 默认初始化一个简单的控制台日志，确保在调用 Init 之前也能用
	logger, _ := zap.NewDevelopment()
	globalLogger = logger.Sugar()
}

// InitLogger 初始化日志
// filePath: 日志文件路径
// maxSize: 每个日志文件最大尺寸 (MB)
// maxBackups: 保留最近多少个文件
// maxAge: 保留最近多少天
func InitLogger(filePath string, maxSize, maxBackups, maxAge int) {
	config := Config{
		FilePath:    filePath,
		MaxSize:     maxSize,
		MaxBackups:  maxBackups,
		MaxAge:      maxAge,
		Level:       "info",
		Compress:    true,
		Development: false,
	}

	globalLogger, _ = InitLoggerWithConfig(config)
}

// InitLoggerWithConfig 使用配置初始化日志
func InitLoggerWithConfig(config Config) (*zap.SugaredLogger, error) {
	// 确保日志目录存在
	dir := filepath.Dir(config.FilePath)
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建日志目录失败: %w", err)
		}
	}

	// 1. 定义日志编码器 (Encoder)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	var encoder zapcore.Encoder
	if config.Development {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// 2. 解析日志级别
	logLevel := zapcore.DebugLevel
	if err := logLevel.Set(config.Level); err != nil {
		return nil, fmt.Errorf("解析日志级别失败: %w", err)
	}

	// 3. 创建不同级别的 Core
	var cores []zapcore.Core

	// --- 文件输出 Core ---
	// INFO 级别及以下
	infoLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		// 当 MsgOnly 开启时，这个 core 不再处理 Info 级别
		if config.MsgOnly && lvl == zapcore.InfoLevel {
			return false
		}
		return lvl >= logLevel && lvl < zapcore.WarnLevel
	})
	infoWriter := getLogWriter(config.FilePath, config.MaxSize, config.MaxBackups, config.MaxAge, config.Compress)
	cores = append(cores, zapcore.NewCore(encoder, infoWriter, infoLevel))

	// WARN 级别及以上
	warnLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= logLevel && lvl >= zapcore.WarnLevel
	})
	errorFilePath := getErrorLogFilePath(config.FilePath)
	errorWriter := getLogWriter(errorFilePath, config.MaxSize, config.MaxBackups, config.MaxAge, config.Compress)
	cores = append(cores, zapcore.NewCore(encoder, errorWriter, warnLevel))

	// --- 控制台输出 Core (如果需要) ---
	if config.Development {
		consoleLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= logLevel
		})
		consoleWriter := zapcore.AddSync(os.Stdout)
		// 开发模式下，控制台输出使用更友好的 ConsoleEncoder
		consoleEncoderConfig := zap.NewDevelopmentEncoderConfig()
		consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		consoleEncoder := zapcore.NewConsoleEncoder(consoleEncoderConfig)

		cores = append(cores, zapcore.NewCore(consoleEncoder, consoleWriter, consoleLevel))
	}

	if config.MsgOnly {
		customFilePath := getCustomLogFilePath(config.FilePath, "msg")
		infoWriter := getLogWriter(customFilePath, config.MaxSize, config.MaxBackups, config.MaxAge, config.Compress)
		// ✅ 核心：创建一个只允许 Info 级别的 Enabler
		infoOnlyEnabler := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl == zapcore.InfoLevel
		})
		cores = append(cores, zapcore.NewCore(
			zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
				MessageKey: "msg", // 只输出 message
			}),
			infoWriter,
			infoOnlyEnabler,
		))
	}

	// 4. 使用 NewTee 合并所有 Core
	core := zapcore.NewTee(cores...)

	// 5. 构建 Logger
	// zap.AddCaller() 用于记录调用位置
	// zap.AddCallerSkip(1) 用于跳过当前函数的封装，以显示真实的调用者
	options := []zap.Option{zap.AddCaller(), zap.AddCallerSkip(1)}
	if config.Development {
		options = append(options, zap.AddStacktrace(zapcore.ErrorLevel))
	} else {
		options = append(options, zap.AddStacktrace(zapcore.PanicLevel))
	}

	logger := zap.New(core, options...)

	return logger.Sugar(), nil
}

// getLogWriter 返回一个 lumberjack.Logger
func getLogWriter(filename string, maxSize, maxBackup, maxAge int, compress bool) zapcore.WriteSyncer {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxSize,
		MaxBackups: maxBackup,
		MaxAge:     maxAge,
		Compress:   compress,
	}
	return zapcore.AddSync(lumberJackLogger)
}

// getErrorLogFilePath 根据主日志文件路径生成错误日志文件路径
// e.g., /path/to/app.log -> /path/to/app.error.log
func getErrorLogFilePath(filePath string) string {
	ext := filepath.Ext(filePath)
	base := strings.TrimSuffix(filePath, ext)
	return base + ".error" + ext
}

// 获得自定义日志路径
func getCustomLogFilePath(filePath, customName string) string {
	ext := filepath.Ext(filePath)
	base := strings.TrimSuffix(filePath, ext)
	return base + "." + customName + ext
}

// Sync 刷新缓存 (建议在 main 退出前 defer 调用)
func Sync() {
	_ = globalLogger.Sync()
}

// --- 以下是极简封装，完全模仿 Printf / Println ---

// Infof 格式化日志 (最常用) -> log.Infof("税率: %.2f", tax)
func Infof(template string, args ...interface{}) {
	globalLogger.Infof(template, args...)
}

// Info 普通日志 -> log.Info("发现新币")
func Info(args ...interface{}) {
	globalLogger.Info(args...)
}

// Infow 结构化日志 (推荐) -> log.Infow("交易成功", "profit", 100, "token", "BNB")
// 既简单，又能生成 {"profit": 100, "token": "BNB"} 这种强类型 JSON 字段
func Infow(msg string, keysAndValues ...interface{}) {
	globalLogger.Infow(msg, keysAndValues...)
}

// Error 错误日志 -> log.Errorf("服务器错误: %v", err)
func Error(args ...interface{}) {
	globalLogger.Error(args...)
}

// Errorf 错误日志
func Errorf(template string, args ...interface{}) {
	globalLogger.Errorf(template, args...)
}

// Errorw 结构化错误日志
func Errorw(msg string, keysAndValues ...interface{}) {
	globalLogger.Errorw(msg, keysAndValues...)
}

// Must 如果 err != nil，则打印 Fatal 日志并直接退出程序 (os.Exit(1))
// msg 是可选的，用于补充说明哪里出错了
func Must(err error, msg ...string) {
	if err == nil {
		return
	}

	output := err.Error()
	if len(msg) > 0 {
		output = msg[0] + ": " + output
	}

	// Fatal 系列函数会自动调用 os.Exit(1)，你不需要自己写 os.Exit
	globalLogger.Fatal(output)
}

// Fatal 手动退出程序
func Fatal(args ...interface{}) {
	globalLogger.Fatal(args...)
}

// Fatalf 手动退出程序，支持格式化
func Fatalf(template string, args ...interface{}) {
	globalLogger.Fatalf(template, args...)
}
