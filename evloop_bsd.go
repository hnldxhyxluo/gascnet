
// +build darwin netbsd freebsd openbsd dragonfly

package gascnet

import (
	"errors"
	"runtime"
	"sync/atomic"
	"syscall"
)

type evloop struct {
	id         int
	kqueuefd    int
	notifyflag int32 //0：没有写 1：已经写了

	evtasks []syscall.Kevent_t
	svrs       []*service
	svrmaxidx  int32 //已经分配的svr最大索引
	asyncqueue *funcqueue
}

func newEvLoop(id int) *evloop {
	kqueuefd, err := syscall.Kqueue()
	if err != nil {
		panic(err)
	}

	_, err = syscall.Kevent(kqueuefd, []syscall.Kevent_t{{
		Ident:  0,
		Filter: syscall.EVFILT_USER,
		Flags:  syscall.EV_ADD | syscall.EV_CLEAR,
	}}, nil, nil)

	if err != nil {
		panic(err)
	}

	return &evloop{id: id, kqueuefd: kqueuefd, svrs: make([]*service, 0, 5), asyncqueue: NewFuncQueue(10, 0)}
}

func (this *evloop) Close() {

}

func (this *evloop) run(lockosthread bool) {
	if lockosthread {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}

	events := make([]syscall.Kevent_t, 128)
	havenotify := false
	for {
		n, err := syscall.Kevent(this.kqueuefd, this.evtasks, events, nil)
		if n == 0 || (n < 0 && (err == syscall.EINTR || err == syscall.EAGAIN)) {
			runtime.Gosched()
			continue
		} else if err != nil {
			panic(err)
		}
		this.evtasks = this.evtasks[:0]

		for i := 0; i < n; i++ {
			fd := int(events[i].Ident)
			//svridx := int32(events[i].Udata)
			svridx := int32(events[i].Data)
			ev := events[i].Filter
			if fd != 0 {
				svr := this.svrs[svridx]
				svr.readwrite(fd, (ev&syscall.EVFILT_READ) != 0, (ev&syscall.EVFILT_WRITE) != 0)
			}else{
				atomic.StoreInt32(&this.notifyflag, 0)
				havenotify = true
			}
		}

		if havenotify {
			this.doasync()
			havenotify = false
		}
	}
}

func (this *evloop) notify(c call) error {
	if !this.asyncqueue.push(c) {
		return errors.New("push queue fail")
	}
	if atomic.CompareAndSwapInt32(&this.notifyflag, 0, 1) {
		_, err := syscall.Kevent(this.kqueuefd, []syscall.Kevent_t{{
			Ident:  0,
			Filter: syscall.EVFILT_USER,
			Fflags: syscall.NOTE_TRIGGER,
		}}, nil, nil)
		return err
	}
	return nil
}

func (this *evloop) mod(fd int, svridx int32, oldread, newread, oldwrite, newwrite bool) {

	if !oldread && newread  {
		this.evtasks = append(this.evtasks,
			syscall.Kevent_t{
				Ident: uint64(fd), 
				Flags: syscall.EV_ADD, 
				Filter: syscall.EVFILT_READ,
				//Udata:    svridx,
				Data:    int64(svridx),
			},
		)
	}else if oldread && !newread {
		this.evtasks = append(this.evtasks,
			syscall.Kevent_t{
				Ident: uint64(fd), 
				Flags: syscall.EV_DELETE, 
				Filter: syscall.EVFILT_READ,
				//Udata:    svridx,
				Data:    int64(svridx),
			},
		)
	}

	if !oldwrite && newwrite {
		this.evtasks = append(this.evtasks,
			syscall.Kevent_t{
				Ident: uint64(fd), 
				Flags: syscall.EV_ADD, 
				Filter: syscall.EVFILT_WRITE,
				//Udata:    svridx,
				Data:    int64(svridx),
			},
		)
	}else if oldwrite && !newwrite {
		this.evtasks = append(this.evtasks,
			syscall.Kevent_t{
				Ident: uint64(fd), 
				Flags: syscall.EV_DELETE, 
				Filter: syscall.EVFILT_WRITE,
				//Udata:    svridx,
				Data:    int64(svridx),
			},
		)
	}
}
