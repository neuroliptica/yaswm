package main

import (
	"bytes"
	"crypto/tls"
	"io/ioutil"
	"mime/multipart"
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

type GetRequest struct {
	RequestInternal
}

type PostRequest struct {
	RequestInternal
	Payload *bytes.Buffer
}

func (internal *RequestInternal) Set(req *http.Request) {
	for key, value := range internal.Headers {
		req.Header.Set(key, value)
	}
	for i := range internal.Cookies {
		req.AddCookie(internal.Cookies[i])
	}
}

func (internal *RequestInternal) Do(req *http.Request) (*http.Response, error) {
	client := &http.Client{
		Timeout: internal.Timeout,
	}
	defaultTransport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MaxVersion:         tls.VersionTLS13,
			InsecureSkipVerify: true,
		},
	}
	client.Transport = defaultTransport
	if internal.Transport != nil {
		client.Transport = internal.Transport
	}
	client.Transport.(*http.Transport).ProxyConnectHeader = req.Header
	return client.Do(req)
}

func (preq *PostRequest) Perform() ([]byte, error) {
	req, err := http.NewRequest(
		"POST",
		preq.Url,
		preq.Payload,
	)
	if err != nil {
		return nil, err
	}
	preq.RequestInternal.Set(req)
	preq.Response, err = preq.RequestInternal.Do(req)
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
	greq.RequestInternal.Set(req)
	greq.Response, err = greq.RequestInternal.Do(req)
	if err != nil {
		return nil, err
	}
	defer greq.Response.Body.Close()
	return ioutil.ReadAll(greq.Response.Body)
}

type FilesForm struct {
	Name  string
	Files map[string][]byte
}

type PostMultipartRequest struct {
	Request PostRequest
	Form    FilesForm
	Params  map[string]string
}

func (preq *PostMultipartRequest) Perform() ([]byte, error) {
	preq.Request.Payload = new(bytes.Buffer)
	writer := multipart.NewWriter(preq.Request.Payload)

	for key, value := range preq.Params {
		err := writer.WriteField(key, value)
		if err != nil {
			writer.Close()
			return nil, err
		}
	}
	for file, cont := range preq.Form.Files {
		part, err := writer.CreateFormFile(preq.Form.Name, file)
		if err != nil {
			writer.Close()
			return nil, err
		}
		part.Write(cont)
	}
	writer.Close()

	req, err := http.NewRequest(
		"POST",
		preq.Request.Url,
		preq.Request.Payload,
	)
	if err != nil {
		return nil, err
	}

	preq.Request.Headers["Content-Type"] = writer.FormDataContentType()
	preq.Request.RequestInternal.Set(req)
	preq.Request.Response, err = preq.Request.RequestInternal.Do(req)
	if err != nil {
		return nil, err
	}

	defer preq.Request.Response.Body.Close()
	return ioutil.ReadAll(preq.Request.Response.Body)
}
