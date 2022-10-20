//go:build darwin || netbsd || freebsd || openbsd || dragonfly
// +build darwin netbsd freebsd openbsd dragonfly

package gascnet

import (
	"errors"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

type evloop struct {
	id         int
	kqueuefd   int
	notifyflag int32 //0：没有写 1：已经写了

	evtasks    []syscall.Kevent_t
	svrs       []*service
	svrmaxidx  int32 //已经分配的svr最大索引
	asyncqueue *funcqueue
}

func newEvLoop(id int, notifyqueuelen int) *evloop {
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

	return &evloop{id: id, kqueuefd: kqueuefd, svrs: make([]*service, 0, 5), asyncqueue: NewFuncQueue(notifyqueuelen, 0)}
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
		//n, err := syscall.Kevent(this.kqueuefd, this.evtasks, events, nil)
		n, err := syscall.Kevent(this.kqueuefd, nil, events, nil)
		if n == 0 || (n < 0 && (err == syscall.EINTR || err == syscall.EAGAIN)) {
			//runtime.Gosched()
			continue
		} else if err != nil {
			panic(err)
		}
		//this.evtasks = this.evtasks[:0]

		for i := 0; i < n; i++ {
			fd := int(events[i].Ident)
			//svridx := *(*int32)(unsafe.Pointer(events[i].Udata))
			//svridx := int32(events[i].Data)
			ev := events[i].Filter
			if fd != 0 {
				//svr := this.svrs[svridx]
				svr := (*service)(unsafe.Pointer(events[i].Udata))
				svr.readwrite(fd, (ev&syscall.EVFILT_READ) != 0, (ev&syscall.EVFILT_WRITE) != 0)
			} else {
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

func (this *evloop) mod(fd int, svr *service, oldread, newread, oldwrite, newwrite bool) {

	var evs [2]syscall.Kevent_t
	count := 0
	if !oldread && newread {
		evs[0].Ident = uint64(fd)
		evs[0].Flags = syscall.EV_ADD
		evs[0].Filter = syscall.EVFILT_READ
		evs[0].Udata = (*byte)(unsafe.Pointer(svr))
		count = 1
	} else if oldread && !newread {
		evs[0].Ident = uint64(fd)
		evs[0].Flags = syscall.EV_DELETE
		evs[0].Filter = syscall.EVFILT_READ
		evs[0].Udata = (*byte)(unsafe.Pointer(svr))
		count = 1
	}

	if !oldwrite && newwrite {
		evs[count].Ident = uint64(fd)
		evs[count].Flags = syscall.EV_ADD
		evs[count].Filter = syscall.EVFILT_WRITE
		evs[count].Udata = (*byte)(unsafe.Pointer(svr))
		count = count + 1
	} else if oldwrite && !newwrite {
		evs[count].Ident = uint64(fd)
		evs[count].Flags = syscall.EV_DELETE
		evs[count].Filter = syscall.EVFILT_WRITE
		evs[count].Udata = (*byte)(unsafe.Pointer(svr))
		count = count + 1
	}
	if count > 0 {
		if _, err := syscall.Kevent(this.kqueuefd, evs[:count], nil, nil); err != nil {
			panic(err)
		}
	}
}

/*
func (this *evloop) mod(fd int, svr *service, oldread, newread, oldwrite, newwrite bool) {

	evs := syscall.Kevent_t[2]
	count := 0
	if !oldread && newread {
		evs[0].Ident = uint64(fd)

		this.evtasks = append(this.evtasks,
			syscall.Kevent_t{
				Ident:  uint64(fd),
				Flags:  syscall.EV_ADD,
				Filter: syscall.EVFILT_READ,
				//Udata:  (*byte)(svridx),
				Udata: (*byte)(unsafe.Pointer(svr)),
			},
		)
	} else if oldread && !newread {
		this.evtasks = append(this.evtasks,
			syscall.Kevent_t{
				Ident:  uint64(fd),
				Flags:  syscall.EV_DELETE,
				Filter: syscall.EVFILT_READ,
				//Udata:  (*byte)(svridx),
				Udata: (*byte)(unsafe.Pointer(svr)),
			},
		)
	}

	if !oldwrite && newwrite {
		this.evtasks = append(this.evtasks,
			syscall.Kevent_t{
				Ident:  uint64(fd),
				Flags:  syscall.EV_ADD,
				Filter: syscall.EVFILT_WRITE,
				//Udata:  (*byte)(svridx),
				Udata: (*byte)(unsafe.Pointer(svr)),
			},
		)
	} else if oldwrite && !newwrite {
		this.evtasks = append(this.evtasks,
			syscall.Kevent_t{
				Ident:  uint64(fd),
				Flags:  syscall.EV_DELETE,
				Filter: syscall.EVFILT_WRITE,
				//Udata:  (*byte)(svridx),
				Udata: (*byte)(unsafe.Pointer(svr)),
			},
		)
	}

	_, err := unix.Kevent(p.fd, evs[:], nil, nil)
}
*/
