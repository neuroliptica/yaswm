package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"time"
)

type RequestInternal struct {
	Url       string
	Headers   map[string]string
	Cookies   []*http.Cookie
	Transport *http.Transport
	Timeout   time.Duration

	Response *http.Response
}

type PostRequest struct {
	RequestInternal
	Payload []byte
}

type GetRequest struct {
	RequestInternal
}

func (preq *PostRequest) Perform() ([]byte, error) {
	req, err := http.NewRequest(
		"POST",
		preq.Url,
		bytes.NewBuffer(preq.Payload),
	)
	if err != nil {
		return nil, err
	}
	for key, value := range preq.Headers {
		req.Header.Set(key, value)
	}
	for i := range preq.Cookies {
		req.AddCookie(preq.Cookies[i])
	}
	client := &http.Client{
		Timeout: preq.Timeout,
	}

	preq.Response, err = client.Do(req)
	if err != nil {
		return nil, err
	}
	defer preq.Response.Body.Close()
	return ioutil.ReadAll(preq.Response.Body)
}

func (greq *GetRequest) Perform() ([]byte, error) {
	req, err := http.NewRequest("GET", greq.Url, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range greq.Headers {
		req.Header.Set(key, value)
	}
	for i := range greq.Cookies {
		req.AddCookie(greq.Cookies[i])
	}
	client := &http.Client{
		Timeout: greq.Timeout,
	}
	greq.Response, err = client.Do(req)
	if err != nil {
		return nil, err
	}
	defer greq.Response.Body.Close()
	return ioutil.ReadAll(greq.Response.Body)
}
