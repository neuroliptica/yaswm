package main

import (
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var MainWg = &sync.WaitGroup{}

func init() {
	if _, err := parser.Parse(); err != nil {
		os.Exit(0)
	}
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if options.InternalOptions.Verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.TimeOnly,
	})
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
		log.Error().Msgf("ошибка конфига: %s", err.Error())
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
