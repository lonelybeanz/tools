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
		Level:       "info",
		Compress:    true,
		Development: false,
		MsgOnly:     true,
	}

	esLogger, _ = log.InitLoggerWithConfig(config)
}

// WriteLog 提供服务的统一日志写入接口
func WriteMsgLog(message string) {
	esLogger.Info(message)
}
