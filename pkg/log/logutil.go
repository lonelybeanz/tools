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
	//
	// [0] = "/Users/dujiao/repositories/projects/scutum/scutum-sync/internal/model/__debug_bin2374299233"
	// [1] = "-test.run"
	// [2] = "^TestSnapshot$"
	// 判断当前程序名是否包含 "test" 或者 "debug"
	args := os.Args
	if len(args) < 2 {
		return false
	}
	if strings.HasSuffix(os.Args[0], ".test") {
		return true
	}
	if args[1] == "-test.run" {
		return true
	}
	return strings.HasSuffix(os.Args[0], "debug")
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
