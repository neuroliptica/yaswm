package main

import (
	"log"
	"os"
	"path/filepath"
	"sync"
)

var logger *Logger

var MainWg = &sync.WaitGroup{}

func init() {
	log.SetFlags(log.Ltime)
	if _, err := parser.Parse(); err != nil {
		os.Exit(0)
	}
	logger = MakeLogger(filepath.Base(os.Args[0])).
		BindChanReader(&defaultReader)
}

func main() {
	go defaultReader.Read()
	defer defaultReader.WaitFinish()

	var env Env
	err := Maybe{
		func() error {
			return env.ParseMedia("./res/images")
		},
		func() error {
			return env.ParseTexts("./res/texts.txt")
		},
		func() error {
			return env.ParseUserAgents("./res/UserAgents.conf")
		},
		func() error {
			return env.ParseProxies("./res/proxies.conf")
		},
		env.ParseWipeMode,
		env.ParseLimiter,
		env.ParseThread,
		env.ParseSolver,
	}.
		Eval()

	if err != nil {
		logger.Log("фатальная ошибка: ", err.Error())
		return
	}

	for i := range env.Proxies {
		unit := Unit{
			Env:     &env,
			Wg:      MainWg,
			Proxy:   env.Proxies[i],
			Headers: make(map[string]string),
			State:   NoCookies,
		}

		if unit.Proxy.Http() && unit.Proxy.Private() {
			unit.Headers["Proxy-Authorization"] = unit.Proxy.AuthHeader()
		}

		MainWg.Add(1)
		go unit.Run()
	}

	MainWg.Wait()
}
