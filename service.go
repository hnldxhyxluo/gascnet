package gascnet

import (
	"errors"
	"fmt"
)

type service struct {
	opt           svroption
	index         int32
	listenbacklog int
	network       string
	ip            string
	port          int
	name          string
	evhandle      EvHandler
	listenfd      int
	conns         map[int]*tcpconn
	count         int32
	loopid        int
	loop          *evloop
}

func (this *service) addconn(c *tcpconn) error {
	_, exist := this.conns[c.fd]
	if exist {
		return errors.New("exist yet")
	}
	this.conns[c.fd] = c
	c.svr = this
	c.svrid = int(this.index)
	c.loopid = this.loopid
	this.loop.mod(c.fd, this, false, c.watchread, false, c.watchwrite)
	return nil
}

func (this *service) Watch(c *tcpconn, watchread, watchwrite bool) (err error) {
	if c.svr != this {
		err = errors.New("svr not compare")
	} else {
		this.loop.mod(c.fd, this, c.watchread, watchread, c.watchwrite, watchwrite)
		c.watchread = watchread
		c.watchwrite = watchwrite
	}
	return
}

func (this *service) remove(c *tcpconn, isneedcall bool) error {
	if c.svr != this {
		return errors.New("not found")
	}

	this.loop.mod(c.fd, this, c.watchread, false, c.watchwrite, false)
	delete(this.conns, c.fd)
	c.svr = nil

	if isneedcall {
		this.evhandle.OnConnClose(this.loopid, c, nil)
	}
	return nil
}

func (this *service) add(c *tcpconn) error {
	if c.svr == this {
		return errors.New("re add")
	}
	this.conns[c.fd] = c
	c.svr = this
	this.loop.mod(c.fd, this, false, c.watchread, false, c.watchwrite)
	return nil
}

func (this *service) readwrite(fd int, canread, canwrite bool) {
	if fd == this.listenfd {
		if canread {
			this.accept()
		}
	} else {
		c, ok := this.conns[fd]
		if ok {
			this.evhandle.OnConnReadWrite(this.loopid, c, canread, canwrite)
		} else {
			this.evhandle.OnServiceErr(this.loop.id, fmt.Errorf("fd:%d not found in this service", fd))
			//this.loop.mod(fd, this, true, false, true, false)
			//socketClose(fd)
		}
	}
}

func (this *service) stop() {
	if this.listenfd >= 0 {
		this.loop.mod(this.listenfd, this, true, false, false, false)
		this.listenfd = -1
	}
}

func (this *service) clean() {
	this.stop()
	for _, c := range this.conns {
		this.evhandle.OnConnClose(this.loopid, c, nil)
		socketClose(c.fd)
		c.isopen = false
		c.fd = -1
		c.svr = nil
		c.loopid = -1
		c.ctx = nil
	}
}
