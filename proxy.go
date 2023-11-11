package main

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
	"unicode"

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

	Localhost bool
}

func (p *Proxy) String() string {
	if p.Localhost {
		return "localhost"
	}
	return p.Ip + ":" + p.Port
}

func (p *Proxy) Http() bool {
	return p.Protocol == "http" || p.Protocol == "https"
}

func (p *Proxy) Socks() bool {
	return !p.Http()
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
	r := net.ParseIP(p.Ip)
	if r == nil {
		return fmt.Errorf("%s invalid format", p)
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
		transport.Proxy = http.ProxyURL(p.AddrParsed)
	}
	return transport
}

func (p *Proxy) CheckAlive() bool {
	return true
}

func (p *Proxy) GetCookies() ([]*http.Cookie, error) {
	return nil, nil
}
