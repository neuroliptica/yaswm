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
		env.ParseWipeMode,
		env.ParseOther,
		func() error { return env.GetMedia("./res") },
		func() error { return env.GetTexts("./res/texts.txt") },
		func() error { return env.GetProxies("./res/proxies.conf") },
	}.
		Eval()

	if err != nil {
		logger.Log(err.Error())
		return
	}

	test := Unit{
		Env:    &env,
		Proxy:  Proxy{Localhost: true}, //env.Proxies[0],
		Logger: logger,
		State:  Avaiable,
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (X11; Linux x86_64; rv:102.0) Gecko/20100101 Firefox/102.0",
		},
	}
	if test.Proxy.Http() && test.Proxy.Private() {
		test.Headers["Proxy-Authorization"] = test.Proxy.AuthHeader()
	}

	test.Run()
}
