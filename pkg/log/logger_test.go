package log

import (
	"testing"
)

func TestInit(t *testing.T) {
	// 测试默认配置初始化
	InitLogger("./logs/test.log", 100, 3, 28)

	// 测试自定义配置初始化
	config := Config{
		FilePath:    "./logs/test_config.log",
		MaxSize:     50,
		MaxBackups:  5,
		MaxAge:      7,
		Level:       "info",
		Compress:    true,
		Development: true,
	}

	if err := InitLoggerWithConfig(config); err != nil {
		t.Fatalf("初始化日志失败: %v", err)
	}
}

func TestLog1(t *testing.T) {

	defer Sync() // 退出前刷盘

	tokenSymbol := "WBNB"
	tax := 0.05

	// 【用法 1】完全兼容 Printf (最顺手)
	// 输出: {"level":"INFO","msg":"检测到代币 WBNB, 税率: 0.05", ...}
	Infof("检测到代币 %s, 税率: %.2f", tokenSymbol, tax)

	// 【用法 2】简单的 Println
	Info("正在初始化 ES 连接...")

	// 【用法 3】Key-Value 模式 (专家推荐 🔥)
	// 虽然写起来像参数列表，但在日志文件里会自动转成 JSON 字段，方便 ES 查询
	// 你不用写 zap.String(...) 这么长，直接 "key", value 即可
	// 输出: {"level":"ERROR","msg":"套利失败","reason":"余额不足","need":10.5,"func":"main.go:20"}
	Errorw("套利失败",
		"reason", "余额不足",
		"need", 10.5,
	)
}
