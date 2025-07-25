package main

import (
	"bytes"
	"crypto/tls"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

type Request struct {
	Url     string
	Payload *bytes.Buffer

	Headers map[string]string
	Cookies []*http.Cookie

	Transport *http.Transport
	Timeout   time.Duration

	Response *http.Response
}

func (r *Request) GetResponse() *http.Response {
	return r.Response
}

func (r *Request) GetRequest() *Request {
	return r
}

type UnitRequest interface {
	Perform() ([]byte, error)

	GetRequest() *Request
	GetResponse() *http.Response
}

func (r *Request) Set(req *http.Request) {
	for key, value := range r.Headers {
		req.Header.Set(key, value)
	}
	for i := range r.Cookies {
		req.AddCookie(r.Cookies[i])
	}
}

func (r *Request) Do(req *http.Request) (*http.Response, error) {
	client := &http.Client{
		Timeout: r.Timeout,
	}
	defaultTransport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MaxVersion:         tls.VersionTLS13,
			InsecureSkipVerify: true,
		},
	}
	client.Transport = defaultTransport
	if r.Transport != nil {
		client.Transport = r.Transport
	}
	client.Transport.(*http.Transport).ProxyConnectHeader = req.Header

	return client.Do(req)
}

func (r *Request) Perform() ([]byte, error) {
	t := "GET"
	var p io.Reader
	if r.Payload != nil {
		t = "POST"
		p = r.Payload
	}

	req, err := http.NewRequest(t, r.Url, p)
	if err != nil {
		return nil, err
	}
	r.Set(req)
	r.Response, err = r.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Response.Body.Close()
	return io.ReadAll(r.Response.Body)
}

type FilesForm struct {
	Name  string
	Files map[string][]byte
}

type PostMultipartRequest struct {
	Request Request
	Form    FilesForm
	Params  map[string]string
}

func (r *PostMultipartRequest) GetResponse() *http.Response {
	return r.Request.Response
}

func (r *PostMultipartRequest) GetRequest() *Request {
	return &r.Request
}

func (r *PostMultipartRequest) Perform() ([]byte, error) {
	r.Request.Payload = new(bytes.Buffer)
	writer := multipart.NewWriter(r.Request.Payload)

	for key, value := range r.Params {
		err := writer.WriteField(key, value)
		if err != nil {
			writer.Close()
			return nil, err
		}
	}
	for file, cont := range r.Form.Files {
		part, err := writer.CreateFormFile(r.Form.Name, file)
		if err != nil {
			writer.Close()
			return nil, err
		}
		part.Write(cont)
	}
	writer.Close()

	req, err := http.NewRequest(
		"POST",
		r.Request.Url,
		r.Request.Payload,
	)
	if err != nil {
		return nil, err
	}

	if r.Request.Headers == nil {
		r.Request.Headers = make(map[string]string)
	}
	r.Request.Headers["Content-Type"] = writer.FormDataContentType()
	r.Request.Set(req)

	r.Request.Response, err = r.Request.Do(req)
	if err != nil {
		return nil, err
	}

	defer r.Request.Response.Body.Close()
	return io.ReadAll(r.Request.Response.Body)
}
