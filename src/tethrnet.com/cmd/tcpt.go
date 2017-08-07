package main

import (
	"github.com/abiosoft/ishell"
	"github.com/qiniu/log"
	"net/http"
	_ "net/http/pprof"

	"tethrnet.com/mgr"
)

func main() {
	log.SetOutputLevel(log.Ldebug)

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	var manager *mgr.Mgr
	started := false
	shell := ishell.New()
	shell.AddCmd(&ishell.Cmd{
		Name: "start",
		Func: func(c *ishell.Context) {
			if started == true {
				log.Error("daemon has been started")
				return
			}
			if len(c.Args) > 1 {
				log.Error("the formate should be like: start [10.0.0.1]")
				return
			}
			if len(c.Args) == 1 {
				manager = mgr.NewMgr(c.Args[0])
			} else {
				manager = mgr.NewMgr("0.0.0.0")
			}
			if manager == nil {
				log.Error("fail to create daemon")
				return
			}
			started = true
			go manager.Run()
		},
		Help: "start the daemon",
	})
	shell.AddCmd(&ishell.Cmd{
		Name: "add",
		Help: "create a new tcp tunnel: add localName rmtName 10.1.1.1(rmt addr)",
		Func: func(c *ishell.Context) {
			if started == false {
				log.Error("daemon has not been started")
				return
			}
			if len(c.Args) != 3 {
				log.Error("format should be like: add localName rmtName 10.1.1.1(rmt addr)")
				return
			}
			manager.CreateTunnel(c.Args[0], c.Args[1], c.Args[2])
		}})

	shell.AddCmd(&ishell.Cmd{
		Name: "del",
		Help: "del a tcp tunnel: del 10.1.1.1 (rmt addr)",
		Func: func(c *ishell.Context) {
			if started == false {
				log.Error("daemon has not been started")
				return
			}
			if len(c.Args) != 1 {
				log.Error("format should be like: del 10.1.1.1(rmt addr)")
				return
			}
			manager.DelTunnel(c.Args[0])
		}})
	shell.AddCmd(appendDebug())
	shell.Run()
}

func appendDebug() *ishell.Cmd {
	cmd := (&ishell.Cmd{
		Name: "debug-lvl",
		Help: "set debue level (debug, info, warn, error) ",
	})
	cmd.AddCmd(&ishell.Cmd{
		Name: "debug",
		Help: "debug < info < warn < error",
		Func: func(c *ishell.Context) {
			log.SetOutputLevel(log.Ldebug)
		}})
	cmd.AddCmd(&ishell.Cmd{
		Name: "info",
		Help: "debug < info < warn < error",
		Func: func(c *ishell.Context) {
			log.SetOutputLevel(log.Linfo)
		}})
	cmd.AddCmd(&ishell.Cmd{
		Name: "warn",
		Help: "debug < info < warn < error",
		Func: func(c *ishell.Context) {
			log.SetOutputLevel(log.Lwarn)
		}})
	cmd.AddCmd(&ishell.Cmd{
		Name: "error",
		Help: "debug < info < warn < error",
		Func: func(c *ishell.Context) {
			log.SetOutputLevel(log.Lerror)
		}})
	return cmd
}
