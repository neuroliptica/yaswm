package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"
)

const (
	CaptchaApi = "/api/captcha/2chcaptcha/"
	PostingApi = "/user/posting"
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
	CaptchaIdError
	CaptchaIdParsingError
	CaptchaImageError
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
	url := fmt.Sprintf(
		"https://2ch.hk%sid?board=%s&thread=%s",
		CaptchaApi,
		unit.Env.Board,
		unit.Env.Thread,
	)
	unit.Log(url)
	req := GetRequest{
		RequestInternal: RequestInternal{
			Url:     url,
			Headers: unit.Headers,
			Cookies: unit.Cookies,
			Timeout: time.Second * 30,
		},
	}
	cont, err := req.Perform()
	if err != nil {
		return UnitError{
			Code:    CaptchaIdError,
			Message: err.Error(),
		}
	}
	var response struct {
		Id     string
		Result int
	}
	json.Unmarshal(cont, &response)
	if response.Id == "" {
		return UnitError{
			Code:    CaptchaIdParsingError,
			Message: string(cont),
		}
	}
	unit.CaptchaId = response.Id
	return nil
}

func (unit *Unit) GetCaptchaImage() ([]byte, error) {
	url := fmt.Sprintf(
		"https://2ch.hk%sshow?id=%s",
		CaptchaApi,
		unit.CaptchaId,
	)
	req := GetRequest{
		RequestInternal{
			Url:     url,
			Headers: unit.Headers,
			Cookies: unit.Cookies,
			Timeout: time.Second * 30,
		},
	}
	return req.Perform()
}

func (unit *Unit) SolveCaptcha() error {
	img, err := unit.GetCaptchaImage()
	if err != nil {
		return UnitError{
			Code:    CaptchaImageError,
			Message: err.Error(),
		}
	}
	ioutil.WriteFile("img.png", img, 0644)
	fmt.Scan(&unit.CaptchaValue)
	return nil
}

func (unit *Unit) GetRandomThread() error {
	return nil
}

func (unit *Unit) SendPost() error {
	url := "https://2ch.hk" + PostingApi
	params := map[string]string{
		"task":             "post",
		"captcha_type":     "2chcaptcha",
		"comment":          unit.Env.Texts[rand.Intn(len(unit.Env.Texts))],
		"board":            unit.Env.Board,
		"thread":           unit.Env.Thread,
		"2chcaptcha_id":    unit.CaptchaId,
		"2chcaptcha_value": unit.CaptchaValue,
	}
	ReqInternal := RequestInternal{
		Url:     url,
		Headers: unit.Headers,
		Cookies: unit.Cookies,
		Timeout: time.Second * 30,
	}
	req := PostMultipartRequest{
		Request: PostRequest{
			RequestInternal: ReqInternal,
		},
		Params: params,
	}
	if len(unit.Env.Media) != 0 {
		file := unit.Env.Media[rand.Intn(len(unit.Env.Media))]
		name := fmt.Sprintf(
			"%d%s",
			time.Now().Unix(),
			file.Ext,
		)
		req.Form = FilesForm{
			Name: "file[]",
			Files: map[string][]byte{
				name: file.Content,
			},
		}
	}
	resp, err := req.Perform()
	if err != nil {
		return err
	}
	unit.Log(string(resp))
	return nil
}

func (unit *Unit) HandleAnswer() error {
	return nil
}
