package asr_client

import (
	"context"
	"fmt"
	"github.com/csh995426531/asr-client/enums"
	"github.com/csh995426531/asr-client/platforms"
	"github.com/csh995426531/asr-client/platforms/tencent"
	"github.com/csh995426531/asr-client/platforms/xunfei"
	"github.com/pkg/errors"
	"io"
)

type ActiveAsr struct {
	platform platforms.Platform
}

var allClient = make(map[enums.Platform]platforms.Platform)
var errPlatformUnEnabled = errors.New("unenabled platforms")

func NewAsr(cfg *Conf) *ActiveAsr {
	if cfg.XunFei.Enable {
		allClient[enums.XunFei] = xunfei.NewXunFei(cfg.XunFei)
	}
	if cfg.Tencent.Enable {
		allClient[enums.Tencent] = tencent.NewTencent(cfg.Tencent)
	}

	active := enums.Platforms.Parse(cfg.ActivePlatform)
	if active == nil {
		fmt.Println("invalid active platforms")
		return nil
	}
	activeClient, exists := allClient[*active]
	if !exists {
		fmt.Println("active platforms unenabled")
		return nil
	}
	return &ActiveAsr{platform: activeClient}
}

func (a *ActiveAsr) ActivatePlatform(p enums.Platform) error {
	activeClient, exists := allClient[p]
	if !exists {
		return errPlatformUnEnabled
	}
	a.platform = activeClient
	return nil
}

func (a *ActiveAsr) ExtractAudio(ctx context.Context, file io.ReadCloser) (string, error) {
	cli, err := a.platform.Conn()
	if err != nil {
		return "", err
	}
	go cli.Send(file)
	return cli.Receive()
}

func (a *ActiveAsr) GetActivatePlatform() enums.Platform {
	return a.platform.Info()
}
