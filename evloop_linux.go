package gascnet

import (
	"errors"
	"runtime"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/unix"
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

var (
	notifyvalue uint64 = 1
	notifydata         = (*(*[8]byte)(unsafe.Pointer(&notifyvalue)))[:]
)

func newEvLoop(id int, notifyqueuelen int) *evloop {
	//epollfd, err := syscall.EpollCreate1(0)
	epollfd, err := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if err != nil {
		panic(err)
	}

	/*
		//notifyfd, _, ret := syscall.Syscall(syscall.SYS_EVENTFD2, 0, 0, 0)
		notifyfd, _, ret := syscall.RawSyscall(syscall.SYS_EVENTFD2, 0, 0, 0)
		if ret != 0 {
			syscall.Close(epollfd)
			panic(errors.New("SYS_EVENTFD2"))
		}*/
	notifyfd, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
	if err != nil {
		unix.Close(epollfd)
		panic(err)
	}

	return &evloop{id: id, epollfd: epollfd, notifyfd: int(notifyfd), svrs: make([]*service, 0, 5), asyncqueue: NewFuncQueue(notifyqueuelen, 0)}
}

func (this *evloop) Close() {

}

func (this *evloop) run(lockosthread bool) {
	if lockosthread {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}

	this.mod(this.notifyfd, nil, false, true, false, false)

	eventn := 128
	events := make([]unix.EpollEvent, eventn)
	havenotify := false
	evbuf := make([]byte, 8)
	for {
		n, err := unix.EpollWait(this.epollfd, events, 100)
		if n == 0 || (n < 0 && (err == unix.EINTR || err == unix.EAGAIN)) {
			//runtime.Gosched()
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
				svr.readwrite(fd, (ev&unix.EPOLLIN) != 0, (ev&unix.EPOLLOUT) != 0)
			} else {
				//先清标记 再读数据
				//atomic.StoreInt32(&this.notifyflag, 0)
				//syscall.Read(this.notifyfd, evbuf[:])
				//log.Printf("loopid:%d epfd:%d notified\n", this.id, this.epollfd)
				havenotify = true
			}
		}

		//if havenotify {
		if havenotify || this.notifyflag != 0 {
			//log.Printf("loopid:%d epfd:%d read notifydata\n", this.id, this.epollfd)
			atomic.StoreInt32(&this.notifyflag, 0)
			unix.Read(this.notifyfd, evbuf)
			this.doasync()
			havenotify = false
		}

		if n == eventn {
			eventn = eventn << 1
			events = make([]unix.EpollEvent, eventn)
		} else if n < (eventn>>1) && eventn > 128 {
			eventn = eventn >> 1
			events = make([]unix.EpollEvent, eventn)
		}
	}
}

func (this *evloop) notify(c call) error {
	if !this.asyncqueue.push(c) {
		return errors.New("push queue fail")
	}

	if atomic.CompareAndSwapInt32(&this.notifyflag, 0, 1) {
		//var x uint64 = 1
		//_, err := syscall.Write(this.notifyfd, (*(*[8]byte)(unsafe.Pointer(&x)))[:])
		if n, err := unix.Write(this.notifyfd, notifydata); err != nil {
			//if err != unix.EAGAIN {
			return err
			//}
		} else if n <= 0 {
			return errors.New("send fail")
		}
	} //else {
	//	log.Printf("loopid:%d epfd:%d no need write notifydata\n", this.id, this.epollfd)
	//}

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
		oper = unix.EPOLL_CTL_ADD
		if newread {
			ev = unix.EPOLLIN
		}
		if newwrite {
			ev = ev | unix.EPOLLOUT
		}
	} else if newread || newwrite {
		oper = unix.EPOLL_CTL_MOD
		if newread {
			ev = unix.EPOLLIN
		}
		if newwrite {
			ev = ev | unix.EPOLLOUT
		}
	} else {
		oper = unix.EPOLL_CTL_DEL
		ev = unix.EPOLLIN | unix.EPOLLOUT
	}

	if err := unix.EpollCtl(this.epollfd, oper, fd,
		&unix.EpollEvent{Fd: int32(fd),
			Events: ev,
			Pad:    index,
		},
	); err != nil {
		panic(err)
	}
}
