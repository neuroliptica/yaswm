package main

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/devices"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"golang.org/x/net/proxy"
)

var protos = map[string]bool{
	"http":   true,
	"https":  true,
	"socks4": true,
	"socks5": true,
}

type Proxy struct {
	Protocol    string
	Ip, Port    string
	Login, Pass string

	Addr      *url.URL
	Localhost bool

	UserAgent string
}

func (p *Proxy) String() string {
	if p.Localhost {
		return "localhost"
	}
	return p.Ip + ":" + p.Port
}

func (p *Proxy) Http() bool {
	return p.Protocol == "http"
}

func (p *Proxy) Socks() bool {
	return p.Protocol == "socks4" || p.Protocol == "socks5"
}

func (p *Proxy) Private() bool {
	return p.Login != "" && p.Pass != ""
}

func (p *Proxy) ParseProto(word *string) error {
	a := strings.Split(*word, "://")
	if len(a) != 2 || !protos[a[0]] {
		return fmt.Errorf("invalid protocol")
	}
	p.Protocol = a[0]
	if p.Protocol == "https" {
		p.Protocol = "http"
	}
	*word = a[1]
	return nil
}

func (p *Proxy) ParseCredits(word *string) error {
	a := strings.Split(*word, "@")
	if len(a) == 1 {
		return nil
	}
	creds := strings.Split(a[0], ":")
	if len(creds) != 2 {
		return fmt.Errorf("invalid credits")
	}
	p.Login, p.Pass = creds[0], creds[1]
	*word = a[1]
	return nil
}

func (p *Proxy) ParseAddr(word *string) error {
	a := strings.Split(*word, ":")
	if len(a) != 2 {
		return fmt.Errorf("invalid addr")
	}
	p.Ip, p.Port = a[0], a[1]
	return nil
}

func (p *Proxy) Parse(word string) error {
	word = string(Filter([]rune(word), func(r rune) bool {
		return !unicode.IsSpace(r)
	}))
	if word == "localhost" {
		p.Localhost = true
		return nil
	}
	err := Maybe{
		func() error { return p.ParseProto(&word) },
		func() error { return p.ParseCredits(&word) },
		func() error { return p.ParseAddr(&word) },
	}.
		Eval()
	if err != nil {
		return err
	}
	p.Addr, err = url.Parse(p.Protocol + "://" + p.String())
	if err != nil {
		return fmt.Errorf("%s invalid format: %v", p, err)
	}
	return nil
}

func (p *Proxy) AuthHeader() string {
	credits := p.Login + ":" + p.Pass
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(credits))
}

func (p *Proxy) Transport() *http.Transport {
	config := &tls.Config{
		MaxVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,
	}
	if p.Localhost {
		return &http.Transport{
			TLSClientConfig: config,
		}
	}
	proto := make(map[string]func(string, *tls.Conn) http.RoundTripper)
	transport := &http.Transport{
		TLSClientConfig:   config,
		TLSNextProto:      proto,
		DisableKeepAlives: true,
	}
	// socks
	if p.Socks() {
		auth := &proxy.Auth{
			User:     p.Login,
			Password: p.Pass,
		}
		if !p.Private() {
			auth = nil
		}
		dialer, _ := proxy.SOCKS5("tcp", p.String(), auth, proxy.Direct)
		transport.Dial = dialer.Dial
	} else {
		// http(s)
		transport.Proxy = http.ProxyURL(p.Addr)
	}
	return transport
}

func (p *Proxy) CheckAlive() bool {
	return true
}

// Screen Structure
type ScreenStructure struct {
	Width  int
	Height int
}

// Gen Random Device Screen
func RandomDeviceScreen() ScreenStructure {
	Screens := []ScreenStructure{
		{1366, 768},
		{1920, 1080},
		{1280, 1024},
		{1600, 900},
		{1380, 800},
		{1024, 768},
		{1440, 900},
	}

	return Screens[rand.Intn(len(Screens))]
}

// Gen Random Device Pixel Ratio
func RandomDevicePixelRatio() float64 {
	PixelRatios := [3]float64{
		1,
		1.25,
		1.5,
	}

	return PixelRatios[rand.Intn(len(PixelRatios))]
}

// Gen Random Device
func RandomDevice(UserAgent string) devices.Device {
	Screen := RandomDeviceScreen()

	return devices.Device{
		Title:          "Windows",
		Capabilities:   []string{},
		UserAgent:      UserAgent,
		AcceptLanguage: "en",
		Screen: devices.Screen{
			DevicePixelRatio: RandomDevicePixelRatio(),
			Horizontal: devices.ScreenSize{
				Width:  Screen.Width,
				Height: Screen.Height,
			},
			Vertical: devices.ScreenSize{
				Width:  Screen.Height,
				Height: Screen.Width,
			},
		},
	}
}

func (p *Proxy) GetCookies() (cookies []*http.Cookie, err error) {
	defer func() {
		if r := recover(); r != nil {
			//logger("[rod-debug] panic!: %v", r)
			err = fmt.Errorf("возникла внутренняя ошибка: %v", r)
		}
	}()

	u := launcher.New().
		Set("--force-webrtc-ip-handling-policy", "disable_non_proxied_udp").
		Set("--enforce-webrtc-ip-permission-check", "False").
		Set("--use-gl", "osmesa").
		MustLaunch()

	browser := rod.New().ControlURL(u).Timeout(2 * time.Minute).MustConnect()
	defer browser.Close()

	Device := RandomDevice(p.UserAgent)
	page := browser.MustPage("")
	page.MustEmulate(Device)
	page.MustSetViewport(Device.Screen.Horizontal.Width, Device.Screen.Horizontal.Height, 0, false)
	page.MustSetExtraHeaders("cache-control", "max-age=0")
	page.MustSetExtraHeaders("sec-ch-ua", `Google Chrome";v="102", "Chromium";v="102", ";Not A Brand";v="102"`)
	page.MustSetExtraHeaders("sec-fetch-site", "same-origin")
	page.MustSetExtraHeaders("sec-fetch-user", "?1")
	page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent:      p.UserAgent,
		AcceptLanguage: "ru-RU,ru;=0.9",
	})
	page.MustEvalOnNewDocument(`localStorage.clear();`)

	router := page.HijackRequests()
	defer router.Stop()

	// ignore
	router.MustAdd("*.jpg", func(ctx *rod.Hijack) {
		ctx.Response.Fail(proto.NetworkErrorReasonAborted)
	})
	router.MustAdd("*.gif", func(ctx *rod.Hijack) {
		ctx.Response.Fail(proto.NetworkErrorReasonAborted)
	})
	router.MustAdd("*.png", func(ctx *rod.Hijack) {
		ctx.Response.Fail(proto.NetworkErrorReasonAborted)
	})
	router.MustAdd("*google*", func(ctx *rod.Hijack) {
		ctx.Response.Fail(proto.NetworkErrorReasonAborted)
	})
	router.MustAdd("*24smi*", func(ctx *rod.Hijack) {
		ctx.Response.Fail(proto.NetworkErrorReasonAborted)
	})
	router.MustAdd("*yadro.ru*", func(ctx *rod.Hijack) {
		ctx.Response.Fail(proto.NetworkErrorReasonAborted)
	})

	router.MustAdd("*", func(ctx *rod.Hijack) {
		transport := p.Transport()
		if p.Http() && p.Private() {
			ctx.Request.Req().Header.Set("Proxy-Authorization", p.AuthHeader())
		}
		if !p.Localhost {
			transport.ProxyConnectHeader = ctx.Request.Req().Header
		}
		client := http.Client{
			Transport: transport,
			Timeout:   2 * time.Minute,
		}
		//logger(fmt.Sprintf("[rod-debug] [%d] %s",
		//	ctx.Response.Payload().ResponseCode, ctx.Request.URL()))
		ctx.LoadResponse(&client, true)
	})
	go router.Run()

	err = page.Navigate("https://2ch.hk/b")
	if err != nil {
		return nil, err
	}
	page.MustWaitNavigation()
	time.Sleep(time.Second * 20)

	captchaUrl := "https://2ch.hk/api/captcha/emoji/id"
	err = page.Navigate(captchaUrl)
	if err != nil {
		return nil, err
	}
	page.MustWaitLoad()

	cookie, err := page.Cookies([]string{captchaUrl})
	if err != nil {
		return nil, err
	}
	for i := range cookie {
		cookies = append(cookies, &http.Cookie{
			Name:   cookie[i].Name,
			Value:  cookie[i].Value,
			Path:   cookie[i].Path,
			Domain: cookie[i].Domain,
		})
	}
	return cookies, nil
}
