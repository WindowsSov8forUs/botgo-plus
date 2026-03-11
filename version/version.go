// Package version sdk 版本声明。
package version

import (
	"fmt"
)

const (
	// version sdk 版本
	version = "v1.0.0"
	sdkName = "BotGoPlusSDK"
)

// String 输出版本号
func String() string {
	return fmt.Sprintf("%s/%s", sdkName, version)
}
