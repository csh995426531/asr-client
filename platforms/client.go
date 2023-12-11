package platforms

import (
	"github.com/csh995426531/asr-client/enums"
	"io"
)

type Platform interface {
	Conn() (Cli, error)
	Info() enums.Platform
}

type Cli interface {
	Send(file io.ReadCloser) (err error)
	Receive() (res string, err error)
}
