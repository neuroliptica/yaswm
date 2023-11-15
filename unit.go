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
	ClosedSingle
)

// Posting stages
const (
	CaptchaId = iota
	CaptchaImage
	CaptchaSolving
	SendingPost
)

var StageName = map[int]string{
	CaptchaId:      "CaptchaId",
	CaptchaImage:   "CaptchaImage",
	CaptchaSolving: "CaptchaSolving",
	SendingPost:    "SendingPost",
}

// UnitError codes
const (
	NetworkError = iota
	ParsingError
	InternalError
)

const (
	ErrorTooFast        = -8
	ErrorClosed         = -7
	ErrorBanned         = -6
	ErrorInvalidCaptcha = -5
	ErrorAccessDenied   = -4
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
		options.PostOptions.Board,
		unit.Env.Thread,
	)
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
			Code:    NetworkError,
			Message: err.Error(),
		}
	}
	if unit.LastAnswer.Response.StatusCode != 200 {
		return UnitError{
			Code:    NetworkError,
			Message: "invalid response code",
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
			Code:    ParsingError,
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
			Code:    NetworkError,
			Message: err.Error(),
		}
	}
	unit.LastAnswer.Body = img
	if unit.LastAnswer.Response.StatusCode != 200 {
		return UnitError{
			Code:    NetworkError,
			Message: "invalid response code",
		}
	}
	unit.LastAnswer = Answer{
		Stage: CaptchaSolving,
	}
	ioutil.WriteFile("img.png", img, 0644)
	fmt.Scan(&unit.CaptchaValue)

	unit.Logf("капча: %s", unit.CaptchaValue)
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
		"board":            options.PostOptions.Board,
		"thread":           unit.Env.Thread,
		"2chcaptcha_id":    unit.CaptchaId,
		"2chcaptcha_value": unit.CaptchaValue,
	}
	ReqInternal := RequestInternal{
		Url:       url,
		Headers:   unit.Headers,
		Cookies:   unit.Cookies,
		Timeout:   time.Second * 60,
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
			time.Now().UnixMilli(),
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
			Code:    NetworkError,
			Message: err.Error(),
		}
	}
	unit.LastAnswer.Body = resp
	if unit.LastAnswer.Response.StatusCode != 200 {
		return UnitError{
			Code:    NetworkError,
			Message: "invalid response code",
		}
	}
	return nil
}

func (unit *Unit) HandleAnswer() (string, error) {
	type Ok struct {
		Num, Result int32
	}

	type Fail struct {
		Error struct {
			Code    int32
			Message string
		}
		Result int32
	}

	var answer any
	msg := string(unit.LastAnswer.Body)

	answer = &Ok{}
	json.Unmarshal(unit.LastAnswer.Body, answer.(*Ok))

	if answer.(*Ok).Num != 0 {
		msg = "OK: " + msg
		return msg, nil
	}

	answer = &Fail{}
	json.Unmarshal(unit.LastAnswer.Body, answer.(*Fail))

	fail := answer.(*Fail)

	if fail.Error.Code == 0 && fail.Error.Message != "" {
		fail.Error.Code = ErrorBanned
	}

	switch fail.Error.Code {

	case ErrorBanned:
		unit.State = Banned

	case ErrorAccessDenied:
		// btw for mobile proxies maybe should filter
		// ErrorAccessDenied <=> country banned?
		unit.State = Banned

	case ErrorClosed:
		unit.State = Avaiable
		if unit.Env.WipeMode == SingleThread {
			unit.State = ClosedSingle
			// will filter this case
		}

	case ErrorInvalidCaptcha, ErrorTooFast:
		break

	case 0: // if Fail{} is empty after parsing
		return msg, UnitError{
			Code:    ParsingError,
			Message: "failed to parse makaba answer",
		}
	}

	msg = fmt.Sprintf("%d: %s", fail.Error.Code, fail.Error.Message)
	return msg, nil
}

func (unit *Unit) HandleError(err UnitError) {
	switch err.Code {
	case NetworkError:
		unit.HandleNetworkError(err)

	case ParsingError:
		unit.HandleParsingError(err)
	}
}

func (unit *Unit) HandleNetworkError(err UnitError) {
	format := `произошла ошибка! дамп ошибки:
{
	error-type: NetworkError;
	message:    %s;
	stage:      %s;
	response:   %v;
}`

	msg := fmt.Sprintf(
		format,
		err.Message,
		StageName[unit.LastAnswer.Stage],
		unit.LastAnswer.Response,
	)
	unit.Log(msg)

	if unit.LastAnswer.Response == nil {
		unit.State = Failed
		return
	}

	switch unit.LastAnswer.Response.StatusCode {
	// Cloudfare codes:
	case 401, 403, 301, 302, 304:
		unit.State = NoCookies

	case 200:
		unit.State = Avaiable

	default:
		unit.State = Failed
	}
}

func (unit *Unit) HandleParsingError(err UnitError) {
	format := `произошла ошибка! дамп ошибки:
{
	error-type: ParsingError;
	message:    %s;
	stage:      %s;
	last-body:  %s;
}`

	msg := fmt.Sprintf(
		format,
		err.Message,
		StageName[unit.LastAnswer.Stage],
		unit.LastAnswer.Body,
	)
	unit.Log(msg)
	// TODO
}
