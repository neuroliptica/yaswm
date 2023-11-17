package main

import (
	"encoding/json"
	"fmt"
	"time"
)

type RuCaptchaResponse struct {
	Status  int32
	Request string
}

type RuCaptchaError struct {
	Response RuCaptchaResponse
	Message  string
}

func (err RuCaptchaError) Error() string {
	return fmt.Sprintf(
		"RuCaptchaError:[status=%d][request=%s]: %s",
		err.Response.Status,
		err.Response.Request,
		err.Message,
	)
}

func RuCaptchaSolver(img []byte, key string) (string, error) {
	answer, err := RuCaptchaPost(img, key)
	if err != nil {
		return "", RuCaptchaError{
			Response: RuCaptchaResponse{},
			Message:  "не удалось отправить капчу: " + err.Error(),
		}
	}
	if answer.Status != 1 {
		return "", RuCaptchaError{
			Response: *answer,
			Message:  "неожиданный ответ солвера",
		}
	}

	errors, limit := 0, 3

	for {
		get, err := RuCaptchaGet(answer.Request, key)

		if err != nil {
			errors++
			if errors >= limit {
				return "", RuCaptchaError{
					Response: RuCaptchaResponse{},
					Message:  "не удалось получить ответ: " + err.Error(),
				}
			}
			continue
		}

		if get.Status == 1 {
			return get.Request, nil
		}

		switch get.Request {
		case "CAPCHA_NOT_READY":
			break

		default:
			return "", RuCaptchaError{
				Response: *get,
				Message:  "ошибка солвера",
			}
		}
		time.Sleep(time.Second * 2)
	}
}

func RuCaptchaPost(img []byte, key string) (*RuCaptchaResponse, error) {
	form := FilesForm{
		Name: "file",
		Files: map[string][]byte{
			"file": img,
		},
	}
	params := map[string]string{
		"method": "post",
		"key":    key,
		"json":   "1",
	}
	ReqInternal := RequestInternal{
		Url:     "http://rucaptcha.com/in.php",
		Timeout: time.Minute,
	}
	req := PostMultipartRequest{
		Request: PostRequest{
			RequestInternal: ReqInternal,
		},
		Params: params,
		Form:   form,
	}
	resp, err := req.Perform()

	if err != nil {
		return nil, err
	}

	answer := &RuCaptchaResponse{}
	json.Unmarshal(resp, answer)

	return answer, nil
}

func RuCaptchaGet(id string, key string) (*RuCaptchaResponse, error) {
	url := "http://rucaptcha.com/res.php?key=" + key +
		"&action=get&json=1&id=" + id

	req := GetRequest{
		RequestInternal: RequestInternal{
			Url:     url,
			Timeout: time.Minute,
		},
	}
	resp, err := req.Perform()

	if err != nil {
		return nil, err
	}

	answer := &RuCaptchaResponse{}
	json.Unmarshal(resp, answer)

	return answer, nil
}
