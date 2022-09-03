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
	index     int
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

	loops := make([]*evloop, 0, egopt.numloops)
	for i := 0; i < egopt.numloops; i++ {
		loop := newEvLoop(i)
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

func (this *engine) AddService(name string, call func(name string, err error), evhandle EvHandler, opts ...SvrOption) error {
	svrname := name
	logic := func() {

		if this.isexit {
			call(svrname, errors.New("engine exited"))
			return
		}

		svropt := &svroption{}
		for _, opt := range opts {
			opt(svropt)
		}

		network, ip, port, err := parseProtoAddr(svropt.protoaddr)
		if err != nil {
			call(svrname, err)
			return
		}

		if svropt.listenbacklog <= 0 {
			svropt.listenbacklog = 128
		}

		pos := len(this.svrs)
		for i := 0; i < pos; i++ {
			svr := this.svrs[i]
			if svr == nil {
				if i < pos {
					pos = i
				}
				continue
			}
			if svr.name == svrname {
				call(svrname, errors.New("service name already exists"))
				return
			}
		}
		if pos == len(this.svrs) {
			dstlen := pos + 10
			svrs := make([]*svrinfo, pos, dstlen)
			for i := 0; i < pos; i++ {
				svrs[i] = this.svrs[i]
			}
			this.svrs = svrs
		}

		info := &svrinfo{
			index:    pos,
			opt:      *svropt,
			network:  network,
			ip:       ip,
			port:     port,
			name:     svrname,
			evhandle: evhandle,
			curstate: svrstateNO,
			dststate: svrstateSTOP,
		}
		this.svrs = append(this.svrs, info)

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
					name:     info.name,
					evhandle: info.evhandle,
					listenfd: -1,
					conns:    make(map[int]*tcpconn),
					count:    0,
					loopid:   i,
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
		call(svrname, nil)
	}

	return this.push(logic)
}

func (this *engine) StartService(name string, call func(name string, err error)) error {

	svrname := name
	logic := func() {
		var info *svrinfo

		for i := 0; i < len(this.svrs); i++ {
			if this.svrs[i] == nil {
				continue
			}
			if this.svrs[i].name == svrname {
				info = this.svrs[i]
				break
			}
		}

		if info == nil {
			call(svrname, errors.New("not found"))
			return
		}

		if info.curstate != info.dststate {
			call(svrname, errors.New("stat is changing"))
			return
		}

		if info.curstate != svrstateSTOP {
			call(svrname, errors.New("stat conflict"))
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
		call(svrname, nil)
	}

	return this.push(logic)
}

func (this *engine) StopService(name string, call func(name string, err error)) error {
	svrname := name
	logic := func() {
		var info *svrinfo

		for i := 0; i < len(this.svrs); i++ {
			if this.svrs[i] == nil {
				continue
			}
			if this.svrs[i].name == svrname {
				info = this.svrs[i]
				break
			}
		}

		if info == nil {
			call(svrname, errors.New("not found"))
			return
		}

		if info.curstate != info.dststate {
			call(svrname, errors.New("stat is changing"))
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
		call(svrname, nil)
	}

	return this.push(logic)
}

func (this *engine) DelService(name string, call func(name string, err error)) error {
	svrname := name
	logic := func() {
		var info *svrinfo

		for i := 0; i < len(this.svrs); i++ {
			if this.svrs[i] == nil {
				continue
			}
			if this.svrs[i].name == name {
				info = this.svrs[i]
				break
			}
		}

		if info == nil {
			call(svrname, errors.New("not found"))
			return
		}

		if info.curstate != info.dststate {
			call(svrname, errors.New("stat is changing"))
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
		call(svrname, nil)
	}

	return this.push(logic)
}

func (this *engine) AddToService(name string, loopid int, conn Conn) error {
	if loopid < 0 || loopid >= this.opt.numloops || conn == nil {
		return errors.New("param invalid")
	}

	c := conn.(*tcpconn)
	if c == nil || !c.isopen {
		return errors.New("conn invalid")
	}
	if name == "" {
		if c.svrid >= 0 && c.svrid < int32(len(this.loops[loopid].svrs)) {
			svr := this.loops[loopid].svrs[c.svrid]
			if svr != nil {
				return svr.addconn(c)
			}
		}
	} else {
		for _, svr := range this.loops[loopid].svrs {
			if svr != nil && svr.name == name {
				return svr.addconn(c)
			}
		}
	}

	return errors.New("service not found")
}
