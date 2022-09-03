package main

import (
	"fmt"
	"gascnet"
	"time"
)

type evhandler struct {
}

func (this *evhandler) OnServiceErr(loopid int, err error) {
	fmt.Printf("evhandler OnServiceErr loopid:%d err:%s\n", loopid, err.Error())
}

func (this *evhandler) OnConnOpen(loopid int, conn gascnet.Conn) {
	conn.Watch(true, false)
	fmt.Printf("evhandler OnConnOpen loopid:%d\n", loopid)
}

func (this *evhandler) OnConnClose(loopid int, conn gascnet.Conn, err error) {
	fmt.Printf("evhandler OnConnClose loopid:%d\n", loopid)
}

func (this *evhandler) OnConnReadWrite(loopid int, conn gascnet.Conn, canread, canwrite bool) {
	fmt.Printf("evhandler OnConnReadWrite loopid:%d  canread:%t canwrite:%t\n", loopid, canread, canwrite)
}

func main() {

	fmt.Printf("------------run-------------------\n")

	eng := gascnet.NewEngine(gascnet.WithLoops(2), gascnet.WithLockThread(true))
	if eng == nil {
		fmt.Printf("eng is nil\n")
		return
	}
	eng.AddTask(0, func(loopid int) {
		fmt.Printf("0 and run at loopid:%d\n", loopid)
	})
	eng.AddTask(1, func(loopid int) {
		fmt.Printf("0 and run at loopid:%d\n", loopid)
	})
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		index := i
		eng.AddTask(i%2, func(loopid int) {
			fmt.Printf("%d and run at loopid:%d\n", index, loopid)
		})
	}

	fmt.Printf("------------start-------------------\n")

	name := "firse service"
	protoaddr := "tcp://0.0.0.0:8082"
	handler := &evhandler{}
	success := false

	if err := eng.AddService(name, func(name string, err error) {
		if err != nil {
			fmt.Printf("call service:%s add err:%s\n", name, err.Error())
		} else {
			fmt.Printf("call service:%s add success\n", name)
			success = true
		}
	}, handler, gascnet.WithProtoAddr(protoaddr), gascnet.WithReusePort(true), gascnet.WithLoadBalance(gascnet.LoadBalanceRR)); err != nil {
		fmt.Printf("service:%s add err:%s\n", protoaddr, err.Error())
	}

	for {
		if success {
			break
		}
		time.Sleep(1 * time.Second)
	}

	success = false
	if err := eng.StartService(name, func(name string, err error) {
		if err != nil {
			fmt.Printf("call service:%s start err:%s\n", name, err.Error())
		} else {
			fmt.Printf("call service:%s start success\n", name)
			success = true
		}
	}); err != nil {
		fmt.Printf("service:%s start err:%s\n", name, err.Error())
	}

	for {
		if success {
			break
		}
		time.Sleep(1 * time.Second)
	}

	time.Sleep(100 * time.Second)

	success = false
	if err := eng.StopService(name, func(name string, err error) {
		if err != nil {
			fmt.Printf("call service:%s stop err:%s\n", name, err.Error())
		} else {
			fmt.Printf("call service:%s stop success\n", name)
			success = true
		}
	}); err != nil {
		fmt.Printf("service:%s stop err:%s\n", name, err.Error())
	}

	for {
		if success {
			break
		}
		time.Sleep(1 * time.Second)
	}

	time.Sleep(100 * time.Second)

	success = false
	if err := eng.DelService(name, func(name string, err error) {
		if err != nil {
			fmt.Printf("call service:%s del err:%s\n", name, err.Error())
		} else {
			fmt.Printf("call service:%s del success\n", name)
			success = true
		}
	}); err != nil {
		fmt.Printf("service:%s del err:%s\n", name, err.Error())
	}

	for {
		if success {
			break
		}
		time.Sleep(1 * time.Second)
	}
	fmt.Printf("------------end-------------------\n")
}
