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

// Wipe modes
const (
	SingleThread = iota
	RandomThreads
	Creating
)

// Anti-captcha
const (
	OCR = iota
	RuCaptcha
	Manual
)

type Options struct {
	WipeOptions struct {
		WipeMode    uint8  `short:"m" long:"mode" description:"режим вайпа\n0 - один тред\n1 - вся доска\n2 - создавать треды\n" default:"1" default-mask:"вся доска" choice:"0" choice:"1" choice:"2"`
		ImageServer string `short:"s" long:"image-server" description:"сервер для получения картинок"`
		Timeout     uint   `short:"T" long:"timeout" description:"перерыв между постами для одной прокси в секундах\n" default:"0"`
		Iters       int    `short:"i" long:"iters" description:"кол-во проходов для одной прокси (-1 - бесконечно)" default:"-1"`
		NoProxy     bool   `short:"l" long:"localhost" description:"не использовать прокси"`
		Schizo      bool   `long:"schizo" description:"генерировать шизотекст на основе постов из треда"`
	} `group:"Wipe options"`

	CaptchaOptions struct {
		AntiCaptcha uint8  `short:"c" long:"captcha" description:"решалка капчи\n0 - нейронка\n1 - RuCaptcha\n2 - вручную\n" default:"0" default-mask:"нейронка" choice:"0" choice:"1" choice:"2"`
		Key         string `short:"k" long:"key" description:"ключ для API антикапчи"`
		OcrServer   string `short:"o" long:"ocr-server" description:"API url нейронки" default:"http://127.0.0.1:7860/api/predict"`
		Solve       bool   `long:"solve" description:"считать результат решения капчи"`
	} `group:"Captcha options"`

	PostOptions struct {
		Board  string `short:"b" long:"board" default:"b" description:"доска"`
		Thread string `short:"t" long:"thread" default:"0" description:"id треда если режим один тред" value-name:"ID"`
		Email  string `short:"e" long:"email" description:"задать значение поля email"`
		Pic    bool   `short:"p" long:"pic" description:"крепить картинку к посту"`
	} `group:"Post options"`

	PicsOptions struct {
		Noise bool `short:"n" long:"noise" description:"добавить шумов на картинку"`
		Crop  bool `short:"C" long:"crop" description:"обрезать картинки"`
		Mask  bool `short:"M" long:"mask" description:"добавлять цветовые маски"`
	} `group:"Pics options"`

	InternalOptions struct {
		InitLimit           int    `short:"I" long:"init-limit" description:"максимальное кол-во параллельно получаемых сессий (-1 - по числу проксей)" default:"1"`
		RequestsFailedLimit uint   `short:"F" long:"max-r-fail" default:"1" description:"максимальное число неудачных запросов для одной прокси до удаления, без учета получения сессии"`
		SessionFailedLimit  uint   `short:"S" long:"max-s-fail" default:"1" description:"максимальное число попыток получить сессию (обойти клауду) для одной прокси до удаления"`
		FilterBanned        bool   `short:"f" long:"filter" description:"удалять прокси после бана"`
		Rod                 string `long:"rod" description:"go-rod internal flag"`
	} `group:"Internal options"`
}

var (
	options Options
	parser  = flags.NewParser(&options, flags.Default)
)

type void = struct{}

type Solver = func([]byte, string) (string, error)

type Media struct {
	Ext     string
	Content []byte
}

type Env struct {
	WipeMode uint8
	Thread   string // 0 if creating

	Proxies    []Proxy // TODO: proxies type
	UserAgents []string
	Texts      []string
	Media      []Media

	Limiter chan void
	Solver  Solver
}

func (env *Env) ParseUserAgents(path string) error {
	cont, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	agents := strings.Split(string(cont), "\n")
	env.UserAgents = Filter(agents, func(agent string) bool {
		return Any([]rune(agent), func(r rune) bool {
			return !unicode.IsSpace(r)
		})
	})

	if len(env.UserAgents) == 0 {
		env.UserAgents = append(
			env.UserAgents,
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/102.0.5005.63 Safari/537.36",
		)
	}

	return nil
}

func (env *Env) ParseProxies(path string) error {
	if options.WipeOptions.NoProxy {
		env.Proxies = append(env.Proxies, Proxy{
			Localhost: true,
		})
		return nil
	}

	cont, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	proxies := strings.Split(string(cont), "\n")
	proxies = Filter(proxies, func(proxy string) bool {
		return Any([]rune(proxy), func(r rune) bool {
			return !unicode.IsSpace(r)
		})
	})

	for _, p := range proxies {
		proxy := Proxy{}
		err = proxy.Parse(p)
		if err != nil {
			logger.Logf("[%s] parsing failed: %v", p, err)
			continue
		}
		env.Proxies = append(env.Proxies, proxy)
	}

	if len(env.Proxies) == 0 {
		return errors.New("no valid proxies found")
	}

	return nil
}

func (env *Env) ParseMedia(path string) error {
	if options.WipeOptions.ImageServer != "" || !options.PostOptions.Pic {
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
				if len(ent.Content) >= 2e7 {
					logger.Logf("%s размер превышает 20mb!", entry[i].Name())
					continue
				}
				env.Media = append(env.Media, ent)
			}
		}
	}
	if options.PostOptions.Pic && len(env.Media) == 0 {
		return errors.New("--pics, но ни одного файла не найдено")
	}
	return nil
}

func (env *Env) ParseTexts(path string) error {
	cont, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	texts := strings.Split(string(cont), "\n\n")
	env.Texts = Filter(texts, func(text string) bool {
		return Any([]rune(text), func(r rune) bool {
			return !unicode.IsSpace(r)
		})
	})

	if len(env.Texts) == 0 {
		env.Texts = append(env.Texts, " ")
	}

	return nil
}

func (env *Env) ParseWipeMode() error {
	env.WipeMode = options.WipeOptions.WipeMode
	return nil
}

func (env *Env) ParseLimiter() error {
	if options.InternalOptions.InitLimit <= 0 {
		options.InternalOptions.InitLimit = len(env.Proxies)
	}
	if options.InternalOptions.InitLimit == 0 {
		options.InternalOptions.InitLimit = 1
	}
	env.Limiter = make(chan void, options.InternalOptions.InitLimit)
	return nil
}

func (env *Env) ParseThread() error {
	env.Thread = options.PostOptions.Thread
	if Any([]rune(env.Thread), func(r rune) bool {
		return !unicode.IsDigit(r)
	}) {
		return errors.New("invalid thread id")
	}

	if options.WipeOptions.WipeMode == SingleThread && env.Thread == "0" {
		return errors.New("режим \"один тред\", но id треда не указан!")
	}
	return nil
}

func (env *Env) ParseSolver() error {
	switch options.CaptchaOptions.AntiCaptcha {

	case OCR:
		env.Solver = NeuralSolver

	case RuCaptcha:
		env.Solver = RuCaptchaSolver
		if options.CaptchaOptions.Key == "" {
			return errors.New("ключь API антикапчи не указан!")
		}

	case Manual:
		env.Solver = func(img []byte, key string) (string, error) {
			var value string
			err := os.WriteFile("captcha.png", img, 0664)
			if err != nil {
				return value, err
			}
			fmt.Scan(&value)

			return value, nil
		}
	}
	return nil
}

func (env *Env) RandomUserAgent() string {
	return env.UserAgents[rand.Intn(len(env.UserAgents))]
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
