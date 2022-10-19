package gascnet

import (
	"errors"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
	//"golang.org/x/sys/unix"
)

type evloop struct {
	id         int
	epollfd    int   // epoll fd
	notifyfd   int   // notify fd
	notifyflag int32 //标记是否已经写notifyfd， 0：没有写 1：已经写了

	svrs       []*service
	svrmaxidx  int32 //已经分配的svr最大索引
	asyncqueue *funcqueue
}

func newEvLoop(id int) *evloop {
	epollfd, err := syscall.EpollCreate1(0)
	if err != nil {
		panic(err)
	}

	//notifyfd, _, ret := syscall.Syscall(syscall.SYS_EVENTFD2, 0, 0, 0)
	notifyfd, _, ret := syscall.RawSyscall(syscall.SYS_EVENTFD2, 0, 0, 0)
	if ret != 0 {
		syscall.Close(epollfd)
		panic(errors.New("SYS_EVENTFD2"))
	}

	return &evloop{id: id, epollfd: epollfd, notifyfd: int(notifyfd), svrs: make([]*service, 0, 5), asyncqueue: NewFuncQueue(10, 0)}
}

func (this *evloop) Close() {

}

func (this *evloop) run(lockosthread bool) {
	if lockosthread {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}

	this.mod(this.notifyfd, nil, false, true, false, false)

	events := make([]syscall.EpollEvent, 128)
	havenotify := false
	var evbuf [8]byte
	for {
		n, err := syscall.EpollWait(this.epollfd, events, 100)
		if n == 0 || (n < 0 && (err == syscall.EINTR || err == syscall.EAGAIN)) {
			runtime.Gosched()
			continue
		} else if err != nil {
			panic(err)
		}

		for i := 0; i < n; i++ {
			fd := int(events[i].Fd)
			svridx := int32(events[i].Pad)
			ev := events[i].Events
			if fd != this.notifyfd {
				svr := this.svrs[svridx]
				svr.readwrite(fd, (ev&syscall.EPOLLIN) != 0, (ev&syscall.EPOLLOUT) != 0)
			} else {
				//先清标记 再读数据
				//atomic.StoreInt32(&this.notifyflag, 0)
				//syscall.Read(this.notifyfd, evbuf[:])
				havenotify = true
			}
		}

		if havenotify {
			atomic.StoreInt32(&this.notifyflag, 0)
			syscall.Read(this.notifyfd, evbuf[:])
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
		var x uint64 = 1
		_, err := syscall.Write(this.notifyfd, (*(*[8]byte)(unsafe.Pointer(&x)))[:])
		return err
	}
	return nil
}

func (this *evloop) mod(fd int, svr *service, oldread, newread, oldwrite, newwrite bool) {
	var ev uint32
	var oper int
	var index int32
	if svr != nil {
		index = svr.index
	}
	if !oldread && !oldwrite {
		if !newread && !newwrite {
			return
		}
		oper = syscall.EPOLL_CTL_ADD
		if newread {
			ev = syscall.EPOLLIN
		}
		if newwrite {
			ev = ev | syscall.EPOLLOUT
		}
	} else if newread || newwrite {
		oper = syscall.EPOLL_CTL_MOD
		if newread {
			ev = syscall.EPOLLIN
		}
		if newwrite {
			ev = ev | syscall.EPOLLOUT
		}
	} else {
		oper = syscall.EPOLL_CTL_DEL
		ev = syscall.EPOLLIN | syscall.EPOLLOUT
	}

	if err := syscall.EpollCtl(this.epollfd, oper, fd,
		&syscall.EpollEvent{Fd: int32(fd),
			Events: ev,
			Pad:    index,
		},
	); err != nil {
		panic(err)
	}
}
