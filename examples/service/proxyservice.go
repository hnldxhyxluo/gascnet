package main

import (
	"fmt"
	"time"

	"gascnet"
)

type ProxyInfo struct {
	front gascnet.Conn
	addr  string
	id    int64

	upbuf     []byte
	upbuflen  int
	uproffset int
	upsoffset int

	downbuf     []byte
	downbuflen  int
	downroffset int
	downsoffset int

	back gascnet.Conn
}

type proxyhandler struct {
}

func (this *proxyhandler) OnServiceErr(loopid int, err error) {
	fmt.Printf("proxyhandler OnServiceErr loopid:%d err:%s\n", loopid, err.Error())
}

func (this *proxyhandler) OnConnOpen(loopid int, conn gascnet.Conn) {
	addr := conn.RemoteAddr()
	back, err := gascnet.Dial("tcp://127.0.0.1:8082")
	if err != nil || back == nil {
		if err != nil {
			fmt.Printf("proxyhandler OnConnOpen loopid:%d addr:%s connect back:%s err:%s\n", loopid, addr, "tcp://127.0.0.1:8082", err.Error())
		} else {
			fmt.Printf("proxyhandler OnConnOpen loopid:%d addr:%s connect back:%s ret nil\n", loopid, addr, "tcp://127.0.0.1:8082")
		}
		conn.Close()
		return
	}

	proxyinfo := &ProxyInfo{
		front: conn,
		addr:  addr,
		id:    time.Now().UnixNano(),

		upbuf:    make([]byte, 1024, 1024),
		upbuflen: 1024,

		downbuf:    make([]byte, 1024, 1024),
		downbuflen: 1024,

		back: back,
	}
	conn.SetCtx(proxyinfo)
	back.SetCtx(proxyinfo)

	//同样的 loopid 确保 front 和 back 在同一个 携程上
	if err := g_eng.AddToService(backservice, loopid, back); err != nil {
		fmt.Printf("proxyhandler OnConnOpen loopid:%d addr:%s add back to loop err:%s\n", loopid, addr, err.Error())
		conn.Close()
		back.Close()
		return
	}

	back.Watch(true, false)
	conn.Watch(true, false)

	fmt.Printf("proxyhandler OnConnOpen loopid:%d addr:%s\n", loopid, addr)
}

func (this *proxyhandler) OnConnClose(loopid int, conn gascnet.Conn, err error) {
	ctx := conn.GetCtx()
	if ctx == nil {
		fmt.Printf("proxyhandler OnConnClose loopid:%d ctx is nil\n", loopid)
		return
	}
	info := ctx.(*ProxyInfo)
	if info.back != nil {
		info.back.Close()
	}
	fmt.Printf("proxyhandler OnConnClose loopid:%d addr:%s\n", loopid, info.addr)
}

func (this *proxyhandler) OnConnReadWrite(loopid int, conn gascnet.Conn, canread, canwrite bool) {
	info := conn.GetCtx().(*ProxyInfo)
	fmt.Printf("proxyhandler OnConnReadWrite loopid:%d canread:%t canwrite:%t addr:%s\n", loopid, canread, canwrite, info.addr)
	back := info.back
	front := info.front
	if canread {
		rlen, err := front.Read(info.upbuf[info.uproffset:info.upbuflen])
		if err != nil || rlen == 0 {
			front.Close()
			return
		}
		if rlen > 0 {
			info.uproffset = info.uproffset + rlen

			//上行缓存满了 暂时关闭上行读
			if info.uproffset == info.upbuflen {
				front.Watch(false, info.downsoffset != info.downroffset)
			}

			//打开back写
			if info.upsoffset != info.uproffset {
				back.Watch(info.downroffset != info.downbuflen, true)
			}
		}
	}

	if canwrite {
		wlen, err := front.Write(info.downbuf[info.downsoffset:info.downroffset])
		if err != nil {
			front.Close()
			return
		}
		if wlen > 0 {
			info.downsoffset = info.downsoffset + wlen
			if info.downsoffset == info.downroffset {
				info.downsoffset = 0
				info.downroffset = 0
				front.Watch(info.uproffset != info.upbuflen, false)
				back.Watch(true, info.upsoffset != info.uproffset)
			}
		}
	}
}

type backhandler struct {
}

func (this *backhandler) OnServiceErr(loopid int, err error) {
	fmt.Printf("backhandler OnServiceErr loopid:%d err:%s\n", loopid, err.Error())
}

func (this *backhandler) OnConnOpen(loopid int, conn gascnet.Conn) {
	info := conn.GetCtx().(*ProxyInfo)
	fmt.Printf("backhandler OnConnOpen loopid:%d frontaddr:%s\n", loopid, info.addr)
}

func (this *backhandler) OnConnClose(loopid int, conn gascnet.Conn, err error) {
	ctx := conn.GetCtx()
	if ctx == nil {
		fmt.Printf("backhandler OnConnClose loopid:%d ctx is nil\n", loopid)
		return
	}
	info := ctx.(*ProxyInfo)
	if info.front != nil {
		info.front.Close()
	}
	fmt.Printf("backhandler OnConnClose loopid:%d frontaddr:%s\n", loopid, info.addr)
}

func (this *backhandler) OnConnReadWrite(loopid int, conn gascnet.Conn, canread, canwrite bool) {
	info := conn.GetCtx().(*ProxyInfo)
	fmt.Printf("backhandler OnConnReadWrite loopid:%d canread:%t canwrite:%t frontaddr:%s\n", loopid, canread, canwrite, info.addr)
	back := info.back
	front := info.front
	if canread {
		rlen, err := back.Read(info.downbuf[info.downroffset:info.downbuflen])
		if err != nil || rlen == 0 {
			back.Close()
			return
		}
		if rlen > 0 {
			info.downroffset = info.downroffset + rlen
			if info.downroffset == info.downbuflen {
				back.Watch(false, info.upsoffset != info.uproffset)
			}

			if info.downsoffset != info.downroffset {
				front.Watch(info.uproffset != info.upbuflen, true)
			}
		}
	}

	if canwrite {
		wlen, err := back.Write(info.upbuf[info.upsoffset:info.uproffset])
		if err != nil {
			back.Close()
			return
		}
		if wlen > 0 {
			info.upsoffset = info.upsoffset + wlen
			if info.upsoffset == info.uproffset {
				info.upsoffset = 0
				info.uproffset = 0
				back.Watch(info.downroffset != info.downbuflen, false)
				front.Watch(true, info.downroffset != info.downsoffset)
			}
		}
	}
}
