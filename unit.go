package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
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
	RandomThread
	SendingPost
)

var StageName = map[int]string{
	CaptchaId:      "CaptchaId",
	CaptchaImage:   "CaptchaImage",
	CaptchaSolving: "CaptchaSolving",
	RandomThread:   "RandomThread",
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

	CaptchaId, CaptchaValue string

	Env   *Env
	State uint8
	Wg    *sync.WaitGroup

	LastAnswer     Answer
	FailedRequests uint
	FailedSessions uint
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
	value, err := unit.Env.Solver(img, options.CaptchaOptions.Key)

	if err != nil {
		return UnitError{
			Code:    ParsingError,
			Message: err.Error(),
		}
	}
	unit.CaptchaValue = value
	unit.Logf("капча: %s", unit.CaptchaValue)

	return nil
}

func (unit *Unit) GetRandomThread() (string, error) {
	url := fmt.Sprintf(
		"https://2ch.hk/%s/catalog.json",
		options.PostOptions.Board,
	)
	req := GetRequest{
		RequestInternal: RequestInternal{
			Url:     url,
			Timeout: time.Second * 30,
		},
	}
	unit.LastAnswer = Answer{
		Stage: RandomThread,
	}

	resp, err := req.Perform()
	unit.LastAnswer.Response = req.RequestInternal.Response

	if err != nil {
		return "", UnitError{
			Code:    NetworkError,
			Message: "не удалось получить случайный тред: " + err.Error(),
		}
	}
	unit.LastAnswer.Body = resp

	type Catalog struct {
		Threads []struct{ Num uint64 }
	}

	cata := Catalog{}
	json.Unmarshal(resp, &cata)

	if len(cata.Threads) == 0 {
		return "", UnitError{
			Code:    ParsingError,
			Message: "не найдено ни одного треда",
		}
	}

	thread := cata.Threads[rand.Intn(len(cata.Threads))].Num
	return strconv.Itoa(int(thread)), nil
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

	if options.WipeOptions.WipeMode == RandomThreads {
		thread, err := unit.GetRandomThread()
		if err != nil {
			return err
		}
		params["thread"] = thread
	}

	for options.WipeOptions.Schizo && options.WipeOptions.WipeMode != Creating {
		posts, err := GetPosts(params["board"], params["thread"])
		if err != nil {
			unit.Logf(
				"[%s/%s]: не удалось получить посты из треда: %s",
				params["board"],
				params["thread"],
				err.Error(),
			)
			break
		}

		params["comment"] = NewChain(posts).BuildText(256)
		break
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

	for options.PostOptions.Pic {
		file, err := unit.Env.RandomMedia()
		if err != nil {
			unit.Logf("не удалось прекрепить файл: %v", err)
			break
		}

		content := file.Content

		for options.PicsOptions.Mask {
			cont, err := AddMask(&file)
			if err != nil {
				unit.Logf("mask: ошибка: %v", err)
				break
			}
			content = cont
			break
		}

		for options.PicsOptions.Noise {
			cont, err := DrawNoise(&Media{Ext: file.Ext, Content: content})
			if err != nil {
				unit.Logf("noise: ошибка: %v", err)
				break
			}
			content = cont
			break
		}

		for options.PicsOptions.Crop {
			cont, err := Crop(&Media{Ext: file.Ext, Content: content})
			if err != nil {
				unit.Logf("crop: ошибка: %v", err)
				break
			}
			content = cont
			break
		}

		name := fmt.Sprintf(
			"%d%s",
			time.Now().UnixMilli(),
			file.Ext,
		)
		req.Form = FilesForm{
			Name: "file[]",
			Files: map[string][]byte{
				name: content,
			},
		}
		break
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

	type OkThread struct {
		Result int32
		Thread uint64
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

	answer = &OkThread{}
	json.Unmarshal(unit.LastAnswer.Body, answer.(*OkThread))

	if answer.(*OkThread).Thread != 0 {
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
	error-type: NetworkError
	message:    %s
	stage:      %s
	response:   %v
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
	error-type: ParsingError
	message:    %s
	stage:      %s
	last-body:  %s
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
