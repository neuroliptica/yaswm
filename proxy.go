package main

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/go-rod/rod"
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

func (p *Proxy) GetCookies() (cookies []*http.Cookie, err error) {
	defer func() {
		if r := recover(); r != nil {
			//logger("[rod-debug] panic!: %v", r)
			err = fmt.Errorf("возникла внутренняя ошибка")
		}
	}()
	browser := rod.New().Timeout(2 * time.Minute).MustConnect()
	defer browser.Close()

	page := browser.MustPage("")
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

	captchaUrl := "https://2ch.hk/api/captcha/2chcaptcha/id?board=b&thread=0"
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
