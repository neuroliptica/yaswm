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

func (env *Env) GetProxies(path string) error {
	cont, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	env.Proxies = strings.Split(string(cont), "\n")
	return nil
}

func (env *Env) GetMedia(path string) error {
	entry, err := os.ReadDir(path)
	if err != nil {
		return err
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
	return nil
}

func (env *Env) GetTexts(path string) error {
	cont, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	env.Proxies = strings.Split(string(cont), "\n\n")
	return nil
}

func (env *Env) ParseWipeMode() error {
	env.WipeMode = uint8(*WipeModeFlag)
	if env.WipeMode > Creating {
		return fmt.Errorf("неправильный режим")
	}
	return nil
}

func (env *Env) ParsePostSettings() {
	env.Board = *BoardFlag
	env.Thread = *ThreadFlag
	env.Email = *EmailFlag
}

func (env *Env) ParsingFailed() bool {
	return len(env.Errors) != 0
}

// Env{}.ParseWipeMode().ParsePostSettings().GetProxies().GetMedia().
