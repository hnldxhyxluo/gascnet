package gascnet

import "sync"

type call func(loopid int)

type funcqueue struct {
	initlen int
	maxlen  int
	calls   []call
	lock    sync.Mutex
}

func NewFuncQueue(initlen, maxlen int) *funcqueue {
	if initlen <= 10 {
		initlen = 10
	}
	if maxlen < initlen && maxlen != 0 {
		maxlen = initlen
	}
	return &funcqueue{
		initlen: initlen,
		maxlen:  maxlen,
		calls:   make([]call, 0, initlen),
	}
}

func (this *funcqueue) push(c call) bool {
	this.lock.Lock()
	if this.maxlen != 0 && len(this.calls) >= this.maxlen {
		this.lock.Unlock()
		return false
	}
	this.calls = append(this.calls, c)
	this.lock.Unlock()
	return true
}

func (this *funcqueue) pullall() []call {
	this.lock.Lock()
	if len(this.calls) > 0 {
		calls := this.calls[:len(this.calls)]
		this.calls = make([]call, 0, this.initlen)
		this.lock.Unlock()
		return calls
	}
	this.lock.Unlock()
	return nil
}
