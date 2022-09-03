//+build darwin netbsd freebsd openbsd dragonfly linux
package gascnet

import (
	"net"
	"syscall"
)

func (this *service) accept() {
	newfd, _, err := syscall.Accept(this.listenfd)
	if err != nil {
		if err != syscall.EAGAIN {
			this.evhandle.OnServiceErr(this.loop.id, err)
		}
		return
	}
	if err := syscall.SetNonblock(newfd, true); err != nil {
		socketClose(newfd)
		this.evhandle.OnServiceErr(this.loop.id, err)
		return
	}
	c := &tcpconn{isopen: true, svrid: this.index, fd: newfd, svr: this, loopid: this.loop.id}
	this.conns[newfd] = c
	this.evhandle.OnConnOpen(this.loopid, c)
}

func (this *service) start() {

	if this.listenfd >= 0 || this.ip == "" {
		return
	}

	/*
		tapaddr, err := net.ResolveTCPAddr(this.network, this.addr)
		if err != nil {
			this.evhandle.OnServiceErr(this.loop.id, err)
			return
		}
		ln, err := net.ListenTCP("tcp", tapaddr)
		if err != nil {
			this.evhandle.OnServiceErr(this.loop.id, err)
			return
		}
		file, err := ln.File()
		if err != nil {
			this.evhandle.OnServiceErr(this.loop.id, err)
			return
		}
		this.ln = ln
		this.listenfd = int(file.Fd())
	*/

	domain := syscall.AF_INET
	if this.network == "tcp6" {
		domain = syscall.AF_INET6
	}
	syscall.ForkLock.RLock()
	fd, err := syscall.Socket(domain, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
	syscall.ForkLock.RUnlock()

	if err != nil {
		this.evhandle.OnServiceErr(this.loop.id, err)
		return
	}

	if err = socketSetReuseAddr(fd, 1); err != nil {
		syscall.Close(fd)
		this.evhandle.OnServiceErr(this.loop.id, err)
		return
	}

	if err = socketSetReusePort(fd, 1); err != nil {
		syscall.Close(fd)
		this.evhandle.OnServiceErr(this.loop.id, err)
		return
	}

	if err = syscall.SetNonblock(fd, true); err != nil {
		syscall.Close(fd)
		this.evhandle.OnServiceErr(this.loop.id, err)
		return
	}

	soaddr := &syscall.SockaddrInet4{Port: this.port}
	copy(soaddr.Addr[:], net.ParseIP(this.ip))
	if err = syscall.Bind(fd, soaddr); err != nil {
		syscall.Close(fd)
		this.evhandle.OnServiceErr(this.loop.id, err)
		return
	}

	// Set backlog size to the maximum
	if err = syscall.Listen(fd, this.opt.listenbacklog); err != nil {
		syscall.Close(fd)
		this.evhandle.OnServiceErr(this.loop.id, err)
		return
	}

	this.listenfd = fd
	this.loop.mod(this.listenfd, this.index, false, true, false, false)
}
