package main

import (
	"fmt"
	"time"

	"gascnet"
)

type EchoInfo struct {
	conn    gascnet.Conn
	addr    string
	id      int64
	buf     []byte
	buflen  int
	roffset int
	soffset int
}

type echohandler struct {
}

func (this *echohandler) OnServiceErr(loopid int, err error) {
	fmt.Printf("echohandler OnServiceErr loopid:%d err:%s\n", loopid, err.Error())
}

func (this *echohandler) OnConnOpen(loopid int, conn gascnet.Conn) {
	conn.Watch(true, false)
	info := &EchoInfo{
		conn: conn,
		addr: conn.RemoteAddr(),
		id:   time.Now().UnixNano(),

		buf:    make([]byte, 1024, 1024),
		buflen: 1024,
	}

	conn.SetCtx(info)
	fmt.Printf("echohandler OnConnOpen loopid:%d addr:%s\n", loopid, info.addr)
}

func (this *echohandler) OnConnClose(loopid int, conn gascnet.Conn, err error) {
	info := conn.GetCtx().(*EchoInfo)
	fmt.Printf("echohandler OnConnClose loopid:%d addr:%s\n", loopid, info.addr)
}

func (this *echohandler) OnConnReadWrite(loopid int, conn gascnet.Conn, canread, canwrite bool) {
	info := conn.GetCtx().(*EchoInfo)
	fmt.Printf("echohandler OnConnReadWrite loopid:%d canread:%t canwrite:%t addr:%s\n", loopid, canread, canwrite, info.addr)
	if canread {
		rlen, err := conn.Read(info.buf[info.roffset:info.buflen])
		if err != nil || rlen == 0 {
			conn.Close()
			return
		}
		fmt.Printf("echohandler OnConnReadWrite loopid:%d rlen:%d\n", loopid, rlen)
		if rlen > 0 {
			info.roffset = info.roffset + rlen
			if info.roffset == info.buflen {
				conn.Watch(false, true)
			} else {
				conn.Watch(false, info.roffset != info.soffset)
			}
		}
	}

	if canwrite {
		wlen, err := conn.Write(info.buf[info.soffset:info.roffset])
		if err != nil {
			conn.Close()
			return
		}
		if wlen > 0 {
			info.soffset = info.soffset + wlen
			if info.soffset == info.roffset {
				info.soffset = 0
				info.roffset = 0
				conn.Watch(true, false)
			}
		}
	}
}
