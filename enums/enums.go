package enums

import "github.com/orsinium-labs/enum"

type FileStatus enum.Member[int]

var (
	FirstFrame    = FileStatus{Value: 0}
	ContinueFrame = FileStatus{Value: 1}
	LastFrame     = FileStatus{Value: 2}
	//ChatModels    = enum.New(OpenAI, XFSpark)
)

type Platform enum.Member[string]

var (
	XunFei    = Platform{Value: "xun_fei"}
	Tencent   = Platform{Value: "tencent"}
	Platforms = enum.New(XunFei, Tencent)
)
