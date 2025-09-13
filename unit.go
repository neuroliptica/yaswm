package main

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	CaptchaApi = "/api/captcha/emoji/"
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
	CaptchaGet
	CaptchaClick
	RandomThread
	SendingPost
)

var StageName = map[int]string{
	CaptchaId:    "CaptchaId",
	CaptchaGet:   "CaptchaGet",
	CaptchaClick: "CaptchaClick",
	RandomThread: "RandomThread",
	SendingPost:  "SendingPost",
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

type Challenge struct {
	// Both in base64 format
	Image    string   `json:"image"`
	Keyboard []string `json:"keyboard"`
}

type Unit struct {
	Proxy   Proxy
	Cookies []*http.Cookie
	Headers map[string]string

	CaptchaId, CaptchaValue string
	DvachChallenge          string
	Captcha                 Challenge
	CaptchaClickStage       int

	Env   *Env
	State uint8
	Wg    *sync.WaitGroup

	LastAnswer     Answer
	FailedRequests uint
	FailedSessions uint
}

//func (unit *Unit) Log(msg ...any) {
//	log.Info().Msgf("[%s] %s", unit.Proxy.String(), fmt.Sprint(msg...))
//}
//
//func (unit *Unit) Logf(format string, msg ...any) {
//	log.Info().Msgf(fmt.Sprintf("[%s] ", unit.Proxy.String())+format, msg...)
//}

// Perform() method wrapper to save debug content between requests.
func (unit *Unit) Perform(req UnitRequest) ([]byte, error) {
	r := req.GetRequest()
	// Make this log to verbose loglevel
	log.Debug().Msgf(
		"%s -> %v",
		unit.Proxy.String(),
		r,
	)
	cont, err := req.Perform()
	unit.LastAnswer.Response = req.GetResponse()
	if err != nil {
		return nil, err
	}
	unit.LastAnswer.Body = cont

	return cont, err
}

// Unit Api funciton's wrappers for Maybe{} chain to track external info.
func (unit *Unit) WithStageInfo(f func() error, stage int) func() error {
	unit.LastAnswer = Answer{
		Stage: stage,
	}
	return f
}

func (unit *Unit) GetCaptchaId() error {
	url := "https://2ch.su" + CaptchaApi + "id"
	req := Request{
		Url:       url,
		Headers:   unit.Headers,
		Cookies:   unit.Cookies,
		Timeout:   time.Second * 30,
		Transport: unit.Proxy.Transport(),
	}
	cont, err := unit.Perform(&req)
	if err != nil {
		return UnitError{NetworkError, err.Error()}
	}
	if unit.LastAnswer.Response.StatusCode != 200 {
		return UnitError{NetworkError, "invalid response code"}
	}

	var response struct {
		Id        string
		Result    int
		Challenge struct {
			Hash, Template string
			Limit          int
		}
	}
	if err := json.Unmarshal(cont, &response); err != nil {
		return UnitError{ParsingError, err.Error()}
	}
	if response.Id == "" {
		return UnitError{ParsingError, "unexpected answer"}
	}

	for i := 0; i <= response.Challenge.Limit; i++ {
		text := strings.Replace(
			response.Challenge.Template,
			"%d",
			fmt.Sprintf("%d", i),
			1,
		)
		hashBytes := sha512.Sum512([]byte(text))
		hashHex := hex.EncodeToString(hashBytes[:])

		if hashHex == response.Challenge.Hash {
			unit.DvachChallenge = strconv.Itoa(i)
			break
		}
	}

	unit.CaptchaId = response.Id
	return nil
}

func (unit *Unit) GetCaptchaImage() error {
	unit.Captcha = Challenge{}
	url := fmt.Sprintf(
		"https://2ch.su%sshow?id=%s",
		CaptchaApi,
		unit.CaptchaId,
	)
	req := Request{
		Url:       url,
		Headers:   unit.Headers,
		Cookies:   unit.Cookies,
		Timeout:   time.Second * 30,
		Transport: unit.Proxy.Transport(),
	}
	cont, err := unit.Perform(&req)
	if err != nil {
		return UnitError{NetworkError, err.Error()}
	}
	if err := json.Unmarshal(cont, &unit.Captcha); err != nil {
		return UnitError{ParsingError, err.Error()}
	}
	if unit.Captcha.Image == "" {
		return UnitError{ParsingError, "unexpected answer"}
	}
	return nil
}

type SolverResp struct {
	Index int `json:"index"`
}

type ClickBody struct {
	Id  string `json:"captchaTokenId"`
	Num int    `json:"emojiNumber"`
}

type ClickResp struct {
	Success string `json:"success"`
}

//func SaveBase64Image(base64Data string, filename string) error {
//	base64Data = strings.TrimPrefix(base64Data, "data:image/png;base64,")
//	data, err := base64.StdEncoding.DecodeString(base64Data)
//	if err != nil {
//		return fmt.Errorf("failed to decode base64: %v", err)
//	}
//
//	err = ioutil.WriteFile(filename, data, 0644)
//	if err != nil {
//		return fmt.Errorf("failed to write file: %v", err)
//	}
//
//	return nil
//}
//
//func (unit *Unit) ManualSolveEmoji(data Challenge) (*SolverResp, error) {
//	SaveBase64Image(data.Image, "captcha.png")
//	for i := range data.Keyboard {
//		SaveBase64Image(data.Keyboard[i], fmt.Sprintf("%d.png", i))
//	}
//	var value int
//	fmt.Scan(&value)
//	return &SolverResp{
//		Index: value,
//	}, nil
//}

func (unit *Unit) SolveEmoji(url string, data Challenge) (*SolverResp, error) {
	p, err := json.Marshal(data)
	if err != nil {
		return nil, UnitError{ParsingError, err.Error()}
	}
	req := Request{
		Url: url,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Timeout: time.Second * 30,
		// Can be posted w/o proxy i guess.
		Transport: unit.Proxy.Transport(),
		Payload:   bytes.NewBuffer(p),
	}
	cont, err := unit.Perform(&req)
	if err != nil {
		return nil, UnitError{NetworkError, err.Error()}
	}
	r := SolverResp{}
	if err := json.Unmarshal(cont, &r); err != nil {
		return nil, UnitError{ParsingError, err.Error()}
	}
	// Solver response body, change to Info() loglevel if needed.
	log.Debug().Msg(string(cont))

	return &r, nil
}

func (unit *Unit) ClickCaptcha() error {
	sresp, err := unit.SolveEmoji(options.CaptchaOptions.EmojiServer, unit.Captcha)
	if err != nil {
		return err
	}
	creq, err := json.Marshal(ClickBody{
		Id:  unit.CaptchaId,
		Num: sresp.Index,
	})
	if err != nil {
		return UnitError{ParsingError, err.Error()}
	}

	req := Request{
		Url:       "https://2ch.su/api/captcha/emoji/click",
		Headers:   unit.Headers,
		Cookies:   unit.Cookies,
		Timeout:   time.Second * 30,
		Transport: unit.Proxy.Transport(),
		Payload:   bytes.NewBuffer(creq),
	}
	cont, err := unit.Perform(&req)
	if err != nil {
		return UnitError{NetworkError, err.Error()}
	}

	cresp := ClickResp{}
	if err := json.Unmarshal(cont, &cresp); err != nil {
		return UnitError{ParsingError, err.Error()}
	}
	if cresp.Success != "" {
		unit.CaptchaValue = cresp.Success
		return nil
	}
	ch := Challenge{}
	if err := json.Unmarshal(cont, &ch); err != nil {
		return UnitError{ParsingError, err.Error()}
	}
	unit.Captcha = ch

	return nil
}

//func (unit *Unit) SolveCaptcha() error {
//	img, err := unit.GetCaptchaImage()
//	if err != nil {
//		return UnitError{
//			Code:    NetworkError,
//			Message: err.Error(),
//		}
//	}
//	unit.LastAnswer.Body = img
//	if unit.LastAnswer.Response.StatusCode != 200 {
//		return UnitError{
//			Code:    NetworkError,
//			Message: "invalid response code",
//		}
//	}
//
//	unit.LastAnswer = Answer{
//		Stage: CaptchaSolving,
//	}
//	value, err := unit.Env.Solver(img, options.CaptchaOptions.Key)
//
//	if err != nil {
//		return UnitError{
//			Code:    ParsingError,
//			Message: err.Error(),
//		}
//	}
//	unit.CaptchaValue = value
//
//	if options.CaptchaOptions.Solve {
//		solved, err := Solve(value)
//		if err != nil {
//			return UnitError{
//				Code:    ParsingError,
//				Message: fmt.Sprintf("не удалось средуцировать капчу: %v", err),
//			}
//		}
//		unit.CaptchaValue = solved
//	}
//
//	unit.Logf("капча: %s", unit.CaptchaValue)
//
//	return nil
//}

func (unit *Unit) GetRandomThread() (string, error) {
	url := fmt.Sprintf(
		"https://2ch.su/%s/catalog.json",
		options.PostOptions.Board,
	)
	req := Request{
		Url:     url,
		Timeout: time.Second * 30,
	}
	resp, err := unit.Perform(&req)
	if err != nil {
		return "", UnitError{
			Code:    NetworkError,
			Message: "не удалось получить случайный тред: " + err.Error(),
		}
	}
	type Catalog struct {
		Threads []struct{ Num uint64 }
	}
	cata := Catalog{}
	if err := json.Unmarshal(resp, &cata); err != nil {
		return "", UnitError{ParsingError, err.Error()}
	}
	if len(cata.Threads) == 0 {
		return "", UnitError{ParsingError, "не найдено ни одного треда"}
	}
	thread := cata.Threads[rand.Intn(len(cata.Threads))].Num

	return strconv.Itoa(int(thread)), nil
}

func (unit *Unit) AddMedia() (*FilesForm, error) {
	file, err := unit.Env.RandomMedia()
	if err != nil {
		return nil, err
	}
	if options.PicsOptions.Crop {
		if err := file.Crop(); err != nil {
			log.Warn().Fields(map[string]interface{}{
				"err": err.Error(),
			}).Msgf(
				"%s -> Crop() не удался",
				unit.Proxy.String(),
			)
		}
	}
	if options.PicsOptions.Mask {
		if err := file.AddMask(); err != nil {
			log.Warn().Fields(map[string]interface{}{
				"err": err.Error(),
			}).Msgf(
				"%s -> AddMask() не удался",
				unit.Proxy.String(),
			)
		}
	}
	if options.PicsOptions.Noise {
		if err := file.DrawNoise(); err != nil {
			log.Warn().Fields(map[string]interface{}{
				"err": err.Error(),
			}).Msgf(
				"%s -> Noise() не удался",
				unit.Proxy.String(),
			)
		}
	}
	name := fmt.Sprintf(
		"%d%s",
		time.Now().UnixMilli(),
		file.Ext,
	)

	return &FilesForm{
		Name: "file[]",
		Files: map[string][]byte{
			name: file.Content,
		},
	}, nil
}

func (unit *Unit) SendPost() error {
	url := "https://2ch.su" + PostingApi
	params := map[string]string{
		"task":             "post",
		"captcha_type":     "emoji_captcha",
		"2ch_challenge":    unit.DvachChallenge,
		"comment":          unit.Env.Texts[rand.Intn(len(unit.Env.Texts))],
		"board":            options.PostOptions.Board,
		"thread":           unit.Env.Thread,
		"emoji_captcha_id": unit.CaptchaValue,
		"email":            options.PostOptions.Email,
	}

	if options.WipeOptions.WipeMode == RandomThreads {
		thread, err := func() (string, error) {
			unit.LastAnswer = Answer{
				Stage: RandomThread,
			}
			return unit.GetRandomThread()
		}()
		if err != nil {
			return err
		}
		params["thread"] = thread
	}

	if options.WipeOptions.Schizo && options.WipeOptions.WipeMode != Creating {
		posts, err := GetPosts(params["board"], params["thread"])
		if err != nil {
			log.Warn().Fields(map[string]interface{}{
				"err": err.Error(),
			}).Msgf(
				"%s -> %s/%s: не удалось получить посты из треда",
				unit.Proxy.String(),
				params["board"],
				params["thread"],
			)
		} else {
			params["comment"] = NewChain(posts).BuildText(256)
		}
	}

	req := PostMultipartRequest{
		Request: Request{
			Url:       url,
			Headers:   unit.Headers,
			Cookies:   unit.Cookies,
			Timeout:   time.Second * 60,
			Transport: unit.Proxy.Transport(),
		},
		Params: params,
	}

	if options.PostOptions.Pic {
		form, err := unit.AddMedia()
		if err != nil {
			log.Warn().Fields(map[string]interface{}{
				"err": err.Error(),
			}).Msgf(
				"%s -> не удалось прекрепить файл",
				unit.Proxy.String(),
			)
		} else {
			req.Form = *form
		}
	}

	if _, err := unit.Perform(&req); err != nil {
		return UnitError{NetworkError, err.Error()}
	}
	if unit.LastAnswer.Response.StatusCode != 200 {
		return UnitError{NetworkError, "invalid response code"}
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
		msg = "ok: " + msg
		return msg, nil
	}

	answer = &OkThread{}
	json.Unmarshal(unit.LastAnswer.Body, answer.(*OkThread))

	if answer.(*OkThread).Thread != 0 {
		msg = "ok: " + msg
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
	log.Error().Fields(map[string]interface{}{
		"type":  "NetworkError",
		"msg":   err.Message,
		"stage": StageName[unit.LastAnswer.Stage],
		"resp":  unit.LastAnswer.Response,
	}).Msgf(
		"%s -> произошла ошибка",
		unit.Proxy.String(),
	)

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
	log.Error().Fields(map[string]interface{}{
		"type":  "ParsingError",
		"msg":   err.Message,
		"stage": StageName[unit.LastAnswer.Stage],
		"body":  unit.LastAnswer.Body,
	}).Msgf(
		"%s -> произошла ошибка",
		unit.Proxy.String(),
	)
}
