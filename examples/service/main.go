package main

import (
	"log"
	"syscall"
	"time"

	"github.com/hnldxhyxluo/gascnet"
)

var (
	echoserviceid  int = 0
	backserviceid  int = 1
	proxyserviceid int = 2
	g_eng        gascnet.Engine
)

func create_service(svrid int, protoaddr string, evhandle gascnet.EvHandler) {
	if protoaddr != "" {
		if err := g_eng.AddService(svrid, func(svrid int, err error) {
			if err != nil {
				log.Printf("call service:%d add err:%s\n", svrid, err.Error())
			} else {
				log.Printf("call service:%d add success\n", svrid)
			}
		}, evhandle, gascnet.WithProtoAddr(protoaddr), gascnet.WithReusePort(true), gascnet.WithLoadBalance(gascnet.LoadBalanceRR)); err != nil {
			log.Printf("service:%d add err:%s\n", svrid, err.Error())
			return
		}
	} else {
		if err := g_eng.AddService(svrid, func(svrid int, err error) {
			if err != nil {
				log.Printf("call service:%d add err:%s\n", svrid, err.Error())
			} else {
				log.Printf("call service:%d add success\n", svrid)
			}
		}, evhandle); err != nil {
			log.Printf("service:%d add err:%s\n", svrid, err.Error())
			return
		}
	}

	if err := g_eng.StartService(svrid, func(svrid int, err error) {
		if err != nil {
			log.Printf("call service:%d start err:%s\n", svrid, err.Error())
		} else {
			log.Printf("call service:%d start success\n", svrid)
		}
	}); err != nil {
		log.Printf("service:%d start err:%s\n", svrid, err.Error())
	}
}

func main() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Printf("------------run-------------------\n")

	log.Printf(syscall.EAGAIN.Error())
	log.Printf(syscall.EINPROGRESS.Error())

	g_eng = gascnet.NewEngine(gascnet.WithLoops(2), gascnet.WithLockThread(true))
	if g_eng == nil {
		log.Printf("eng is nil\n")
		return
	}
	g_eng.AddTask(0, func(loopid int) {
		log.Printf("0 and run at loopid:%d\n", loopid)
	})
	g_eng.AddTask(1, func(loopid int) {
		log.Printf("0 and run at loopid:%d\n", loopid)
	})
	for i := 0; i < 5; i++ {
		time.Sleep(1 * time.Second)
		index := i
		g_eng.AddTask(i%g_eng.LoopNum(), func(loopid int) {
			log.Printf("%d and run at loopid:%d\n", index, loopid)
		})
	}

	log.Printf("------------start-------------------\n")

	create_service(echoserviceid, "tcp://0.0.0.0:8082", &echohandler{})
	create_service(backserviceid, "", &backhandler{})
	create_service(proxyserviceid, "tcp://0.0.0.0:8083", &proxyhandler{})

	for {
		time.Sleep(1 * time.Second)
	}
	log.Printf("------------end-------------------\n")
}
