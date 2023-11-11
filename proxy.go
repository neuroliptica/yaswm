package main

import (
	"fmt"
	"net"
	"strings"
	"unicode"
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
}

func (p *Proxy) String() string {
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
