//go:build darwin || netbsd || freebsd || openbsd || dragonfly || linux
// +build darwin netbsd freebsd openbsd dragonfly linux

package gascnet

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func socketClose(fd int) {
	syscall.Close(fd)
}

func socketSetNonblock(fd int, value bool) error {
	return syscall.SetNonblock(fd, value)
}

func socketSetNoDelay(fd, nodelay int) error {
	return unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, nodelay)
}

func socketSetWriteBuffer(fd, size int) error {
	return unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_RCVBUF, size)
}

func socketSetReadBuffer(fd, size int) error {
	return unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_SNDBUF, size)
}

func socketSetReusePort(fd, reuseport int) error {
	return unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEPORT, reuseport)
}

func socketSetReuseAddr(fd, reuseaddr int) error {
	return unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEADDR, reuseaddr)
}

func socketSetLinger(fd, sec int) error {
	lin := unix.Linger{0, 0}
	if sec >= 0 {
		lin.Onoff = 1
		lin.Linger = int32(sec)
	}
	return unix.SetsockoptLinger(fd, syscall.SOL_SOCKET, syscall.SO_LINGER, &lin)
}

func socketSetKeepAlive(fd, secs, probes int) error {
	/*if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, 0x8, 1); err != nil {
		return err
	}
	switch err := syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, 0x101, secs); err {
	case nil, syscall.ENOPROTOOPT: // OS X 10.7 and earlier don't support this option
	default:
		return err
	}
	//return syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_KEEPALIVE, secs)
	return nil
	*/

	if secs <= 0 {
		return nil
	}
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_KEEPALIVE, 1); err != nil {
		return err
	}
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_KEEPINTVL, secs); err != nil {
		return err
	}
	//设置探测次数
	if probes > 0 {
		if err := unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_KEEPCNT, probes); err != nil {
			return err
		}
	}

	return unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_KEEPIDLE, secs)
}
