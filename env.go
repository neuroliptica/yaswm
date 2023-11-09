package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var (
	WipeModeFlag = flag.Uint("mode", RandomThreads, "set up wipe mode.")
	BoardFlag    = flag.String("board", "b", "set up board.")
	ThreadFlag   = flag.String("thread", "", "set up thread id.")
	EmailFlag    = flag.String("email", "", "set up email field value.")
)

// Wipe modes.
const (
	SingleThread = iota
	RandomThreads
	Creating
)

type PostSettings struct {
	Board, Thread, Email string
	// Thread = 0 when creating
}

type Env struct {
	WipeMode uint8
	PostSettings

	Proxies      []string // TODO: proxies type
	Media, Texts []string

	Errors []error
}

func (env *Env) GetProxies(path string) *Env {
	cont, err := os.ReadFile(path)
	if err != nil {
		env.Errors = append(env.Errors, err)
		return env
	}
	env.Proxies = strings.Split(string(cont), "\n")
	return env
}

func (env *Env) GetMedia(path string) *Env {
	entry, err := os.ReadDir(path)
	if err != nil {
		env.Errors = append(env.Errors, err)
		return env
	}
	exts := []string{
		".jpg",
		".png",
		".jpeg",
		".webm",
		".mp4",
		".gif",
	}
	for i := range entry {
		for j := range exts {
			if strings.HasSuffix(entry[i].Name(), exts[j]) {
				env.Media = append(env.Media, entry[i].Name())
			}
		}
	}
	return env
}

func (env *Env) GetTexts(path string) *Env {
	cont, err := os.ReadFile(path)
	if err != nil {
		env.Errors = append(env.Errors, err)
		return env
	}
	env.Proxies = strings.Split(string(cont), "\n\n")
	return env
}

func (env *Env) ParseWipeMode() *Env {
	env.WipeMode = uint8(*WipeModeFlag)
	if env.WipeMode > Creating {
		env.Errors = append(
			env.Errors,
			fmt.Errorf("неправильный режим"),
		)
	}
	return env
}

func (env *Env) ParsePostSettings() *Env {
	env.Board = *BoardFlag
	env.Thread = *ThreadFlag
	env.Email = *EmailFlag
	return env
}

func (env *Env) ParsingFailed() bool {
	return len(env.Errors) != 0
}

// Env{}.ParseWipeMode().ParsePostSettings().GetProxies().GetMedia().
