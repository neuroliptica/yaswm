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
	env.
		ParseWipeMode().
		ParsePostSettings().
		GetMedia("dsajdl").
		GetTexts("dasd").
		GetProxies("dsad")

	if env.ParsingFailed() {
		for _, err := range env.Errors {
			logger.Log(err.Error())
		}
		return
	}
}
