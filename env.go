package main

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"unicode"

	"github.com/jessevdk/go-flags"
)

type Options struct {
	WipeOptions struct {
		WipeMode    uint8  `short:"m" long:"mode" description:"режим вайпа\n0 - один тред\n1 - вся доска\n2 - создавать треды\n" default:"0" default-mask:"вся доска" choice:"0" choice:"1" choice:"2"`
		ImageServer string `short:"s" long:"image-server" description:"сервер для получения картинок"`
		Timeout     uint   `short:"T" long:"timeout" description:"перерыв между постами для одной прокси в секундах" default:"0"`
	} `group:"Wipe options"`

	PostOptions struct {
		Board  string `short:"b" long:"board" default:"b" description:"доска"`
		Thread string `short:"t" long:"thread" default:"0" description:"id треда если режим один тред" value-name:"id"`
		Email  string `short:"e" long:"email" description:"задать значение поля email"`
	} `group:"Post options"`

	InternalOptions struct {
		InitLimit           uint32 `short:"I" long:"init-limit" description:"максимальное кол-во параллельно получаемых сессий" default:"1"`
		RequestsFailedLimit uint   `short:"F" long:"max-r-fail" default:"1" description:"максимальное число неудачных запросов для одной прокси до удаления, без учета получения сессии"`
		SessionFailedLimit  uint   `short:"S" long:"max-s-fail" default:"1" description:"максимальное число попыток получить сессию (обойти клауду) для одной прокси до удаления"`
		FilterBanned        bool   `short:"f" long:"filter" description:"удалять прокси после бана"`

		Verbose bool `short:"v" long:"verbose" description:"дополнительные логи"`
	} `group:"Internal options"`
}

var (
	options Options
	parser  = flags.NewParser(&options, flags.Default)
)

// Wipe modes.
const (
	SingleThread = iota
	RandomThreads
	Creating
)

type void = struct{}

type Media struct {
	Ext     string
	Content []byte
}

type Env struct {
	WipeMode uint8
	Thread   string // 0 if creating

	Proxies []Proxy // TODO: proxies type
	Texts   []string
	Media   []Media

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
	if options.WipeOptions.ImageServer != "" {
		return nil
	}
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
	env.WipeMode = options.WipeOptions.WipeMode
	if env.WipeMode > Creating {
		return fmt.Errorf("неправильный режим")
	}
	return nil
}

func (env *Env) ParseOther() error {
	if options.InternalOptions.InitLimit == 0 {
		return fmt.Errorf("-I cannot be below 1")
	}
	env.Limiter = make(chan void, options.InternalOptions.InitLimit)
	env.Thread = options.PostOptions.Thread
	return nil
}

func (env *Env) RandomMedia() (Media, error) {
	if options.WipeOptions.ImageServer == "" {
		if len(env.Media) == 0 {
			return Media{}, errors.New("empty medias array")
		}
		return env.Media[rand.Intn(len(env.Media))], nil
	}
	req := GetRequest{
		RequestInternal: RequestInternal{
			Url: options.WipeOptions.ImageServer,
		},
	}

	resp, err := req.Perform()
	if err != nil {
		return Media{}, err
	}

	types := map[string]string{
		"image/png":  ".png",
		"image/jpeg": ".jpg",
	}

	ctype := req.RequestInternal.Response.Header.Get("Content-Type")
	logger.Log(req.RequestInternal.Response.Header)

	if ctype == "" || types[ctype] == "" {
		return Media{}, errors.New("invalid Content-Type header")
	}
	if req.RequestInternal.Response.StatusCode != 200 {
		return Media{}, errors.New("invalid response code")
	}

	return Media{
		Ext:     types[ctype],
		Content: resp,
	}, nil
}
