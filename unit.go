package main

import (
	"fmt"
	"net/http"
)

// States
const (
	Avaiable = iota
	NoCookies
	Banned
	Failed
)

// UnitError codes
const (
	InternalError = iota
	BannedError
	NoCookiesError
)

type UnitError struct {
	Code    int
	Message string
}

func (e UnitError) Error() string {
	return fmt.Sprintf("UnitError[code=%d] %s", e.Code, e.Message)
}

type Unit struct {
	Proxy   any // todo
	Cookies []*http.Cookie
	Headers map[string]string

	Env    *Env
	Logger *Logger

	State             uint8
	CurrentExternalIp string
	PrevExternalIp    string

	CaptchaId    string
	CaptchaValue string

	LastAnswer  any // todo
	FailedTimes int
}

func (unit *Unit) Log(msg ...any) {
	unit.Logger.Log(msg...)
}

func (unit *Unit) Logf(format string, msg ...any) {
	unit.Logger.Logf(format, msg...)
}

func (unit *Unit) GetCaptchaId() error {
	return nil
}

func (unit *Unit) GetRandomThread() error {
	return nil
}

func (unit *Unit) SolveCaptcha() error {
	return nil
}

func (unit *Unit) SendPost() error {
	return nil
}

func (unit *Unit) HandleAnswer() error {
	return nil
}
