package main

import (
	"flag"
	"log"
)

func init() {
	flag.Parse()
	log.SetFlags(log.Ltime)
}

func main() {
	go defaultReader.Read()
	defer defaultReader.WaitFinish()

	logger := MakeLogger("main").BindChanReader(&defaultReader)

	var env Env
	err := Maybe{
		env.ParseWipeMode,
		func() error {
			env.ParsePostSettings()
			return nil
		},
		env.ParseOther,
		func() error { return env.GetMedia("./res") },
		func() error { return env.GetTexts("./res/texts.txt") },
		//func() error { return env.GetProxies("adsas") },
	}.Eval()

	if err != nil {
		logger.Log(err.Error())
		return
	}

	test := Unit{
		Env:    &env,
		Logger: logger,
		State:  Avaiable,
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (X11; Linux x86_64; rv:102.0) Gecko/20100101 Firefox/102.0",
		},
	}
	test.Run()
}
