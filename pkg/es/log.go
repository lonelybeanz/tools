package es

import (
	"github.com/lonelybeanz/tools/pkg/log"
	"go.uber.org/zap"
)

var esLogger *zap.SugaredLogger

// 初始化日志系统
func init() {
	// 测试自定义配置初始化
	config := log.Config{
		FilePath:    "./logs/es.log",
		MaxSize:     50,
		MaxBackups:  5,
		MaxAge:      7,
		Level:       "debug",
		Compress:    true,
		Development: false,
		MsgOnly:     true,
	}

	esLogger, _ = log.InitLoggerWithConfig(config)
}

// 用于记录BLK语句
func WriteMsgLog(message string) {
	esLogger.Info(message)
}

// 用于记录ERROR日志
func WriteErrorLog(message string) {
	esLogger.Error(message)
}

// 用于记录DEBUG日志，调试
func WriteDebugLog(message string) {
	esLogger.Debug(message)
}
