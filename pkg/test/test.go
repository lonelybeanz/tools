package test

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func ProjectRoot() string {
	dir, _ := os.Getwd()

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("go.mod not found")
		}
		dir = parent
	}
}

func InitEnvForTest(fun func()) {
	root := ProjectRoot()
	_ = godotenv.Load(filepath.Join(root, ".env"))
	fun()
}
