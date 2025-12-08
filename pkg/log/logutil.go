package log

import (
	"fmt"
	"os"
	"strings"
)

var (
	// 当运行 go test 时，会自动设置此标志
	IsTest = isInTest() || os.Getenv("DEBUG") == "1"
)

// 自动检测是否在测试模式
func isInTest() bool {
	// 判断当前程序名是否包含 "test"
	return strings.HasSuffix(os.Args[0], ".test")
}

// Debugf 只在测试时打印
func Debugf(format string, args ...interface{}) {
	if IsTest {
		msg := fmt.Sprintf(format, args...)
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		fmt.Printf("[DEBUG] %s", msg)
	}
}
