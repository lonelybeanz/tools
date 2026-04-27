package es

import (
	"sync/atomic"
	"time"

	"github.com/lonelybeanz/tools/pkg/log"
	"go.uber.org/zap"
)

var esLogger *zap.SugaredLogger

const msgLogQueueSize = 1024

var (
	msgLogQueue        = make(chan []byte, msgLogQueueSize)
	msgLogSlots        = make(chan struct{}, msgLogQueueSize)
	droppedMsgLogCount atomic.Int64
)

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
	for i := 0; i < msgLogQueueSize; i++ {
		msgLogSlots <- struct{}{}
	}
	go writeMsgLogWorker(msgLogQueue)
}

func writeMsgLogWorker(queue <-chan []byte) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case message := <-queue:
			esLogger.Info(string(message))
			msgLogSlots <- struct{}{}
		case <-ticker.C:
			flushDroppedMsgLogCount()
		}
	}
}

func tryQueueMsgLogBytes(queue chan<- []byte, slots <-chan struct{}, message []byte) bool {
	select {
	case <-slots:
		copied := append([]byte(nil), message...)
		queue <- copied
		return true
	default:
		return false
	}
}

// 用于记录BLK语句
func WriteMsgLog(message string) {
	WriteMsgLogBytes([]byte(message))
}

func WriteMsgLogBytes(message []byte) {
	if !tryQueueMsgLogBytes(msgLogQueue, msgLogSlots, message) {
		droppedMsgLogCount.Add(1)
	}
}

func flushDroppedMsgLogCount() {
	dropped := droppedMsgLogCount.Swap(0)
	if dropped > 0 {
		esLogger.Warnf("bulk log queue is full, dropped %d messages", dropped)
	}
}

// 用于记录ERROR日志
func WriteErrorLog(message string) {
	esLogger.Error(message)
}

// 用于记录DEBUG日志，调试
func WriteDebugLog(message string) {
	esLogger.Debug(message)
}
