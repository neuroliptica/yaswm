package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

type OcrPostBody struct {
	Data []string `json:"data"`
}

type OcrResponse struct {
	Data          []string
	Is_generating bool
	Duration      float64
}

func NeuralSolver(img []byte, key string) (string, error) {
	body := OcrPostBody{
		Data: []string{base64.StdEncoding.EncodeToString(img)},
	}
	payload, err := json.Marshal(body)

	if err != nil {
		return "", err
	}

	req := Request{
		Payload: bytes.NewBuffer(payload),
		Url:     options.CaptchaOptions.OcrServer,
		Timeout: time.Second * 30,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Connection":   "keep-alive",
		},
	}
	cont, err := req.Perform()

	if err != nil {
		return "", err
	}

	resp := OcrResponse{}
	json.Unmarshal(cont, &resp)

	if len(resp.Data) == 0 {
		return "", errors.New("failed to parse answer: " + string(cont))
	}

	return resp.Data[0], nil
}
