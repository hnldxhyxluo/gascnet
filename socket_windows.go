package gascnet

import (
	"golang.org/x/sys/windows"
)

func socketClose(fd int) {

}

func socketSetNonblock(fd int, value bool) error {
	return nil
}

func socketSetNoDelay(fd, nodelay int) error {
	return windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_TCP, windows.TCP_NODELAY, nodelay)
}

func socketSetWriteBuffer(fd, size int) error {
	return windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_RCVBUF, size)
}

func socketSetReadBuffer(fd, size int) error {
	return windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_SNDBUF, size)
}

func socketSetReusePort(fd, reuse int) error {
	return windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, reuse)
}

func socketSetReuseAddr(fd, reuse int) error {
	return windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, reuse)
}

func socketSetLinger(fd, sec int) error {
	return nil
}

func socketSetKeepAlive(fd, secs int) error {
	return nil
}
