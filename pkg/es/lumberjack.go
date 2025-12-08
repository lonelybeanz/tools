package es

import (
	"log"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

var Savelogger *lumberjack.Logger

var Updatelogger *lumberjack.Logger

// 初始化日志系统
func init() {
	// 设置日志存储路径和管理策略
	Savelogger = &lumberjack.Logger{
		Filename:   "./logs/save.log", // 日志文件路径
		MaxSize:    5,                 // 单个文件最大大小（MB）
		MaxBackups: 3,                 // 最大保留旧文件数量
		MaxAge:     7,                 // 日志文件最大保存天数
		Compress:   true,              // 是否压缩旧日志文件
	}

	Updatelogger = &lumberjack.Logger{
		Filename:   "./logs/update.log", // 日志文件路径
		MaxSize:    5,                   // 单个文件最大大小（MB）
		MaxBackups: 3,                   // 最大保留旧文件数量
		MaxAge:     7,                   // 日志文件最大保存天数
		Compress:   true,                // 是否压缩旧日志文件
	}

	// 检查目录是否存在，不存在则创建
	if _, err := os.Stat("./logs"); os.IsNotExist(err) {
		err := os.MkdirAll("./logs", os.ModePerm)
		if err != nil {
			log.Fatalf("Failed to create logs directory: %v\n", err)
		}
	}
}

// WriteLog 提供服务的统一日志写入接口
func WriteSaveLog(message string) {
	_, err := Savelogger.Write([]byte(message))
	if err != nil {
		log.Printf("Failed to write log: %v\n", err)
	}
}

func WriteUpdateLog(message string) {
	_, err := Updatelogger.Write([]byte(message))
	if err != nil {
		log.Printf("Failed to write log: %v\n", err)
	}
}
