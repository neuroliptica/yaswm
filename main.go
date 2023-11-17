package main

import (
	"log"
	"os"
	"path/filepath"
)

var logger *Logger

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
			return env.ParseMedia("./res")
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

	test := Unit{
		Env:   &env,
		Proxy: env.Proxies[0],
		State: Avaiable,
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (X11; Linux x86_64; rv:102.0) Gecko/20100101 Firefox/102.0",
		},
	}
	if test.Proxy.Http() && test.Proxy.Private() {
		test.Headers["Proxy-Authorization"] = test.Proxy.AuthHeader()
	}

	test.Run()
}
