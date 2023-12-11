package asr_client

import (
	"github.com/csh995426531/asr-client/platforms/tencent"
	"github.com/csh995426531/asr-client/platforms/xunfei"
)

type Conf struct {
	ActivePlatform string        `json:"active_platform"`
	XunFei         *xunfei.Conf  `json:"xun_fei"`
	Tencent        *tencent.Conf `json:"tencent"`
}
