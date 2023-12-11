package asr_client

import (
	"context"
	"fmt"
	"github.com/csh995426531/asr-client/platforms/tencent"
	"github.com/csh995426531/asr-client/platforms/xunfei"
	"os"
	"testing"
)

var testFile, _ = os.Open("test.wav")

func TestRun(t *testing.T) {
	t.Run("xunfei", func(t *testing.T) {
		cli := NewAsr(&Conf{
			ActivePlatform: "tencent",
			XunFei: &xunfei.Conf{
				Enable:    true,
				HostURL:   "wss://iat-api.xfyun.cn/v2/iat",
				APPID:     "111",
				APISecret: "222",
				APIKey:    "333",
			},
			Tencent: &tencent.Conf{
				Enable:          true,
				HostURL:         "asr.cloud.tencent.com",
				APPID:           "111",
				SecretID:        "222",
				SecretKey:       "333",
				EngineModelType: "16k_zh",
			},
		})
		text, err := cli.ExtractAudio(context.Background(), testFile)
		fmt.Print(text, err)
	})
}
