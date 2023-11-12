package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"unicode"
)

var (
	WipeModeFlag        = flag.Uint("mode", RandomThreads, "set up wipe mode.")
	BoardFlag           = flag.String("board", "b", "set up board.")
	ThreadFlag          = flag.String("thread", "0", "set up thread id.")
	EmailFlag           = flag.String("email", "", "set up email field value.")
	InitLimitFlag       = flag.Uint("I", 1, "maximum web drivers running at once")
	RequestsFailedLimit = flag.Uint("F", 1, "maximum failed requests")
	SessionsFailedLimit = flag.Uint("S", 1, "maximum failed session requests")
	FilterBanned        = flag.Bool("filter", false, "filter banned proxies")
)

// Wipe modes.
const (
	SingleThread = iota
	RandomThreads
	Creating
)

type void = struct{}

type PostSettings struct {
	Board, Thread, Email string
	// Thread = 0 when creating
}

type Env struct {
	WipeMode uint8
	PostSettings

	Proxies []Proxy // TODO: proxies type
	Texts   []string
	Media   []struct {
		Ext     string
		Content []byte
	}

	Limiter chan void
}

func (env *Env) GetProxies(path string) error {
	cont, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	proxies := strings.Split(string(cont), "\n")
	for _, p := range proxies {
		proxy := Proxy{}
		err = proxy.Parse(p)
		if err != nil {
			logger.Logf("[%s] parsing failed: %v", p, err)
			continue
		}
		env.Proxies = append(env.Proxies, proxy)
	}
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
				ent := struct {
					Ext     string
					Content []byte
				}{}
				ent.Ext = exts[j]
				ent.Content, err = os.ReadFile(
					path + "/" + entry[i].Name(),
				)
				if err != nil {
					return err
				}
				env.Media = append(env.Media, ent)
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
	texts := strings.Split(string(cont), "\n\n")
	for _, text := range texts {
		if Any([]rune(text), func(r rune) bool {
			return !unicode.IsSpace(r)
		}) {
			env.Texts = append(env.Texts, text)
		}
	}
	if len(env.Texts) == 0 {
		env.Texts = append(env.Texts, " ")
	}
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

func (env *Env) ParseOther() error {
	if *InitLimitFlag == 0 {
		return fmt.Errorf("-I cannot be below 1")
	}
	env.Limiter = make(chan void, *InitLimitFlag)
	return nil
}
