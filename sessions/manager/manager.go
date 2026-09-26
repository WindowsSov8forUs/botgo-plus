// Package manager 实现 session manager 所需要的公共方法。
package manager

import (
	"time"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/errs"
)

// CanNotResumeErrSet 不能进行 resume 操作的错误码
var CanNotResumeErrSet = map[int]bool{
	errs.CodeConnCloseCantResume: true,
}

// CanNotIdentifyErrSet 不能进行 identify 操作的错误码
var CanNotIdentifyErrSet = map[int]bool{
	errs.CodeConnCloseCantIdentify: true,
}

// concurrencyTimeWindowSec 并发时间窗口，单位秒
const concurrencyTimeWindowSec = 5

// CalcInterval 根据并发要求，计算连接启动间隔
func CalcInterval(maxConcurrency uint32) time.Duration {
	if maxConcurrency == 0 {
		maxConcurrency = 1
	}
	window := time.Duration(concurrencyTimeWindowSec) * time.Second
	concurrency := time.Duration(maxConcurrency)
	// Round up so a fractional interval never exceeds QQ's five-second quota.
	return (window + concurrency - 1) / concurrency
}

// CanNotResume 是否是不能够 resume 的错误
func CanNotResume(err error) bool {
	e := errs.Error(err)
	if flag, ok := CanNotResumeErrSet[e.Code()]; ok {
		return flag
	}
	return false
}

// CanNotIdentify 是否是不能够 identify 的错误
func CanNotIdentify(err error) bool {
	e := errs.Error(err)
	if flag, ok := CanNotIdentifyErrSet[e.Code()]; ok {
		return flag
	}
	return false
}

// CheckSessionLimit 检查链接数是否达到限制，如果达到限制需要等待重置
func CheckSessionLimit(apInfo *dto.WebsocketAP) error {
	if apInfo.Shards > apInfo.SessionStartLimit.Remaining {
		return errs.ErrSessionLimit
	}
	return nil
}
