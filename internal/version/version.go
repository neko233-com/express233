package version

import "fmt"

// 由 -ldflags 注入；本地 go build 默认为 dev。
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String 完整版本信息。
func String(component string) string {
	return fmt.Sprintf("%s %s (commit=%s, built=%s)", component, Version, Commit, Date)
}
