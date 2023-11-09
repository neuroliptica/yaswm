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
	logger.Log("hello world")
	logger.Log("sieg heil")

	var env Env
	err := Maybe{
		env.ParseWipeMode,
		func() error {
			env.ParseWipeMode()
			return nil
		},
		func() error { return env.GetMedia("ad") },
		func() error { return env.GetTexts("ADASD") },
		func() error { return env.GetProxies("adsas") },
	}.Eval()

	if err != nil {
		logger.Log(err.Error())
	}

}
