package main

import (
	"log"
	"syscall"
	"time"

	"gascnet"
)

var (
	echoservice  string = "echo"
	proxyservice string = "proxy"
	backservice  string = "back"
	g_eng        gascnet.Engine
)

func create_service(servicename string, protoaddr string, evhandle gascnet.EvHandler) {
	if protoaddr != "" {
		if err := g_eng.AddService(servicename, func(name string, err error) {
			if err != nil {
				log.Printf("call service:%s add err:%s\n", name, err.Error())
			} else {
				log.Printf("call service:%s add success\n", name)
			}
		}, evhandle, gascnet.WithProtoAddr(protoaddr), gascnet.WithReusePort(true), gascnet.WithLoadBalance(gascnet.LoadBalanceRR)); err != nil {
			log.Printf("service:%s add err:%s\n", servicename, err.Error())
			return
		}
	} else {
		if err := g_eng.AddService(servicename, func(name string, err error) {
			if err != nil {
				log.Printf("call service:%s add err:%s\n", name, err.Error())
			} else {
				log.Printf("call service:%s add success\n", name)
			}
		}, evhandle); err != nil {
			log.Printf("service:%s add err:%s\n", servicename, err.Error())
			return
		}
	}

	if err := g_eng.StartService(servicename, func(name string, err error) {
		if err != nil {
			log.Printf("call service:%s start err:%s\n", name, err.Error())
		} else {
			log.Printf("call service:%s start success\n", name)
		}
	}); err != nil {
		log.Printf("service:%s start err:%s\n", servicename, err.Error())
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

	create_service(echoservice, "tcp://0.0.0.0:8082", &echohandler{})
	create_service(backservice, "", &backhandler{})
	create_service(proxyservice, "tcp://0.0.0.0:8083", &proxyhandler{})

	for {
		time.Sleep(1 * time.Second)
	}
	log.Printf("------------end-------------------\n")
}
