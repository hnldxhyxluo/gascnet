package gascnet

import (
	"errors"
)

type evloop struct {
	id int

	notifyfd   int   // notify fd
	notifyflag int32 //标记是否已经写notifyfd， 0：没有写 1：已经写了

	svrs       []*service
	svrmaxidx  int32 //已经分配的svr最大索引
	asyncqueue *funcqueue
}

func newEvLoop(id int) *evloop {
	return nil
}

func (this *evloop) Close() {

}

func (this *evloop) run(lockosthread bool) {

}

func (this *evloop) notify(c call) error {
	return errors.New("push queue fail")
}

func (this *evloop) mod(fd int, svridx int32, oldread, newread, oldwrite, newwrite bool) {

}
