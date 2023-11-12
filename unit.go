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
	SessionFailed
)

// Posting stages
const (
	CaptchaId = iota
	CaptchaImage
	CaptchaSolving
	SendingPost
)

// UnitError codes
const (
	InternalError = iota
	BannedError
	NoCookiesError
	CaptchaIdError
	CaptchaIdParsingError
	CaptchaImageError
	SendPostError
)

type Answer struct {
	Stage    int
	Body     []byte
	Response *http.Response
}

type UnitError struct {
	Code    int
	Message string
}

func (e UnitError) Error() string {
	return fmt.Sprintf("UnitError[code=%d] %s", e.Code, e.Message)
}

type Unit struct {
	Proxy   Proxy
	Cookies []*http.Cookie
	Headers map[string]string

	Env    *Env
	Logger *Logger

	State                             uint8
	CurrentExternalIp, PrevExternalIp string

	CaptchaId, CaptchaValue string

	FailedRequests, FailedSessions uint

	LastAnswer   Answer
	BanTimestamp time.Time
}

func (unit *Unit) Log(msg ...any) {
	logger.Log(fmt.Sprintf("[%s] %s", unit.Proxy.String(), fmt.Sprint(msg...)))
}

func (unit *Unit) Logf(format string, msg ...any) {
	logger.Logf(fmt.Sprintf("[%s] ", unit.Proxy.String())+format, msg...)
}

func (unit *Unit) GetCaptchaId() error {
	url := fmt.Sprintf(
		"https://2ch.hk%sid?board=%s&thread=%s",
		CaptchaApi,
		unit.Env.Board,
		unit.Env.Thread,
	)
	unit.Log(url)
	unit.LastAnswer = Answer{
		Stage: CaptchaId,
	}
	req := GetRequest{
		RequestInternal: RequestInternal{
			Url:       url,
			Headers:   unit.Headers,
			Cookies:   unit.Cookies,
			Timeout:   time.Second * 30,
			Transport: unit.Proxy.Transport(),
		},
	}
	cont, err := req.Perform()
	unit.LastAnswer.Response = req.RequestInternal.Response
	if err != nil {
		return UnitError{
			Code:    CaptchaIdError,
			Message: err.Error(),
		}
	}
	unit.LastAnswer.Body = cont
	var response struct {
		Id     string
		Result int
	}
	json.Unmarshal(cont, &response)
	if response.Id == "" {
		return UnitError{
			Code:    CaptchaIdParsingError,
			Message: "unexpected answer",
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
	unit.LastAnswer = Answer{
		Stage: CaptchaImage,
	}
	req := GetRequest{
		RequestInternal{
			Url:       url,
			Headers:   unit.Headers,
			Cookies:   unit.Cookies,
			Timeout:   time.Second * 30,
			Transport: unit.Proxy.Transport(),
		},
	}
	img, err := req.Perform()
	unit.LastAnswer.Response = req.RequestInternal.Response
	return img, err
}

func (unit *Unit) SolveCaptcha() error {
	img, err := unit.GetCaptchaImage()
	if err != nil {
		return UnitError{
			Code:    CaptchaImageError,
			Message: err.Error(),
		}
	}
	unit.LastAnswer.Body = img
	if unit.LastAnswer.Response.StatusCode != 200 {
		return UnitError{
			Code: CaptchaImageError,
			Message: fmt.Sprintf(
				"server response: %s",
				unit.LastAnswer.Response.Status,
			),
		}
	}
	unit.LastAnswer = Answer{
		Stage: CaptchaSolving,
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
		Url:       url,
		Headers:   unit.Headers,
		Cookies:   unit.Cookies,
		Timeout:   time.Second * 30,
		Transport: unit.Proxy.Transport(),
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
			time.Now().UnixNano(),
			file.Ext,
		)
		req.Form = FilesForm{
			Name: "file[]",
			Files: map[string][]byte{
				name: file.Content,
			},
		}
	}
	unit.LastAnswer = Answer{
		Stage: SendingPost,
	}
	resp, err := req.Perform()
	unit.LastAnswer.Response = req.Request.RequestInternal.Response
	if err != nil {
		return UnitError{
			Code:    SendPostError,
			Message: err.Error(),
		}
	}
	unit.LastAnswer.Body = resp
	return nil
}

func (unit *Unit) HandleAnswer() error {
	return nil
}
