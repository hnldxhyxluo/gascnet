package gascnet

import (
	"errors"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type svrstate int32

const (
	svrstateNO    = 0
	svrstateSTOP  = 1
	svrstateSTART = 2
	svrstateDEL   = 3
)

type svrinfo struct {
	opt       svroption
	index     int32
	network   string
	ip        string
	port      int
	name      string
	protoaddr string
	evhandle  EvHandler
	curstate  svrstate //当前状态
	dststate  svrstate //目标状态
}

type engine struct {
	opt    engoption
	svrs   []*svrinfo
	loops  []*evloop
	lock   sync.RWMutex
	ctrlch chan func()
	isexit bool
}

func newEngine(opts ...EngOption) Engine {
	egopt := &engoption{}
	for _, opt := range opts {
		opt(egopt)
	}

	if egopt.numloops <= 0 {
		cpus := int(runtime.NumCPU())
		if egopt.numloops == 0 {
			egopt.numloops = cpus
		} else if egopt.numloops = egopt.numloops + cpus; egopt.numloops <= 0 {
			egopt.numloops = 1
		}
	}

	if egopt.notifyqueuelen < 64 {
		egopt.notifyqueuelen = 64
	}

	loops := make([]*evloop, 0, egopt.numloops)
	for i := 0; i < egopt.numloops; i++ {
		loop := newEvLoop(i, egopt.notifyqueuelen)
		if loop == nil {
			return nil
		}
		loops = append(loops, loop)
		go loop.run(egopt.lockthread)
	}
	eng := &engine{
		opt:    *egopt,
		svrs:   make([]*svrinfo, 0, 10),
		loops:  loops,
		ctrlch: make(chan func(), 100),
	}
	go eng.run()
	return eng
}

func (this *engine) LoopNum() int {
	return this.opt.numloops
}

func (this *engine) AddTask(loopid int, call func(loopid int)) error {
	if loopid >= this.opt.numloops || loopid < 0 {
		return errors.New("loopidx out range")
	}
	return this.loops[loopid].notify(call)
}

func (this *engine) run() {
	ticker := time.NewTicker(time.Duration(1) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case call := <-this.ctrlch:
			call()

		case <-ticker.C:
			if this.isexit {
				break
			}
		}
	}
}

func (this *engine) push(logic func()) error {
	select {
	case this.ctrlch <- logic:
		return nil
	default:
		return errors.New("task is full")
	}
}

func (this *engine) getservice(loopid int, svrid int32) *service {
	if loopid >= 0 && loopid < this.opt.numloops {
		loop := this.loops[loopid]
		if svrid >= 0 && svrid < loop.svrmaxidx {
			return loop.svrs[svrid]
		}
	}
	return nil
}

func parseProtoAddr(protoAddr string) (network, ip string, port int, err error) {
	if protoAddr == "" {
		return
	}
	if strings.Contains(protoAddr, "://") {
		network = strings.Split(protoAddr, "://")[0]
		ipport := strings.Split(protoAddr, "://")[1]
		if strings.Contains(ipport, ":") {
			ip = strings.Split(ipport, ":")[0]
			port, _ = strconv.Atoi(strings.Split(ipport, ":")[1])
		}
	}
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		err = errors.New("proto not support")
	}
	return
}

func (this *engine) AddService(svrid int, call func(svrid int, err error), evhandle EvHandler, opts ...SvrOption) error {
	logic := func() {

		if this.isexit {
			call(svrid, errors.New("engine exited"))
			return
		}

		if svrid < 0 {
			call(svrid, errors.New("svrid < 0"))
			return
		}

		if svrid < len(this.svrs) && this.svrs[svrid] != nil {
			call(svrid, errors.New("svrid already exist"))
			return
		}

		svropt := &svroption{}
		for _, opt := range opts {
			opt(svropt)
		}

		network, ip, port, err := parseProtoAddr(svropt.protoaddr)
		if err != nil {
			call(svrid, err)
			return
		}

		if svropt.listenbacklog <= 0 {
			svropt.listenbacklog = 128
		}

		if svrid >= len(this.svrs) {
			wantlen := svrid/10 + 10
			svrs := make([]*svrinfo, wantlen, wantlen)
			for i, v := range this.svrs {
				svrs[i] = v
			}
			this.svrs = svrs
		}

		info := &svrinfo{
			index:    int32(svrid),
			opt:      *svropt,
			network:  network,
			ip:       ip,
			port:     port,
			evhandle: evhandle,
			curstate: svrstateNO,
			dststate: svrstateSTOP,
		}
		this.svrs[svrid] = info

		var wg sync.WaitGroup
		wg.Add(this.opt.numloops)
		for i := 0; i < this.opt.numloops; i++ {
			loop := this.loops[i]
			loop.notify(func(loopid int) {
				svr := &service{
					opt:      info.opt,
					network:  info.network,
					index:    int32(info.index),
					ip:       info.ip,
					port:     info.port,
					evhandle: info.evhandle,
					listenfd: -1,
					conns:    make(map[int]*tcpconn),
					count:    0,
					loopid:   loopid,
					loop:     loop,
				}
				if int(info.index) >= len(loop.svrs) {
					svrs := make([]*service, (info.index/10+1)*10, (info.index/10+1)*10)
					for j := 0; j < len(loop.svrs); j++ {
						svrs[j] = loop.svrs[j]
					}
					loop.svrs = svrs
				}
				loop.svrs[info.index] = svr
				wg.Done()
			})
		}
		wg.Wait()
		info.curstate = svrstateSTOP
		call(svrid, nil)
	}

	return this.push(logic)
}

func (this *engine) StartService(svrid int, call func(svrid int, err error)) error {

	logic := func() {

		if svrid >= len(this.svrs) || svrid < 0 {
			call(svrid, errors.New("svrid invalid"))
			return
		}
		if this.svrs[svrid] == nil {
			call(svrid, errors.New("svrid not found"))
		}

		info := this.svrs[svrid]

		if info.curstate != info.dststate {
			call(svrid, errors.New("stat is changing"))
			return
		}

		if info.curstate != svrstateSTOP {
			call(svrid, errors.New("stat conflict"))
			return
		}

		info.dststate = svrstateSTART

		var wg sync.WaitGroup

		for i := 0; i < this.opt.numloops; i++ {
			wg.Add(1)
			loop := this.loops[i]
			loop.notify(func(loopid int) {
				loop.svrs[info.index].start()
				wg.Done()
			})
			wg.Wait()
		}
		info.curstate = svrstateSTART
		call(svrid, nil)
	}

	return this.push(logic)
}

func (this *engine) StopService(svrid int, call func(svrid int, err error)) error {

	logic := func() {

		if svrid >= len(this.svrs) || svrid < 0 {
			call(svrid, errors.New("svrid invalid"))
			return
		}
		if this.svrs[svrid] == nil {
			call(svrid, errors.New("svrid not found"))
		}

		info := this.svrs[svrid]

		if info.curstate != info.dststate {
			call(svrid, errors.New("stat is changing"))
			return
		}

		info.dststate = svrstateSTOP

		var wg sync.WaitGroup
		wg.Add(this.opt.numloops)
		for i := 0; i < this.opt.numloops; i++ {
			loop := this.loops[i]
			loop.notify(func(loopid int) {
				loop.svrs[info.index].stop()
				wg.Done()
			})
		}
		wg.Wait()
		info.curstate = svrstateSTOP
		call(svrid, nil)
	}

	return this.push(logic)
}

func (this *engine) DelService(svrid int, call func(svrid int, err error)) error {

	logic := func() {

		if svrid >= len(this.svrs) || svrid < 0 {
			call(svrid, errors.New("svrid invalid"))
			return
		}
		if this.svrs[svrid] == nil {
			call(svrid, errors.New("svrid not found"))
		}

		info := this.svrs[svrid]

		if info.curstate != info.dststate {
			call(svrid, errors.New("stat is changing"))
			return
		}

		info.dststate = svrstateDEL

		var wg sync.WaitGroup
		wg.Add(this.opt.numloops)
		for i := 0; i < this.opt.numloops; i++ {
			loop := this.loops[i]
			loop.notify(func(loopid int) {
				loop.svrs[info.index].clean()
				loop.svrs[info.index] = nil
				wg.Done()
			})
		}
		wg.Wait()
		info.curstate = svrstateDEL
		this.svrs[info.index] = nil
		call(svrid, nil)
	}

	return this.push(logic)
}

func (this *engine) AddToService(loopid, svrid int, conn Conn) error {
	if loopid < 0 || loopid >= this.opt.numloops || svrid < 0 || svrid >= len(this.svrs) || conn == nil {
		return errors.New("param invalid")
	}

	if this.svrs[svrid] == nil {
		return errors.New("svrid not exist")
	}

	if info := this.svrs[svrid]; info.curstate != info.dststate || info.curstate != svrstateSTART {
		return errors.New("svrid not start")
	}

	c := conn.(*tcpconn)
	if c == nil || !c.isopen {
		return errors.New("conn invalid")
	}

	return this.loops[loopid].svrs[svrid].addconn(c)
}
