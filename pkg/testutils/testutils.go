package testutils

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
)

type LoggingTransport struct {
	Transport http.RoundTripper
}

func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.logResponse(t.logRequest(req).Transport.RoundTrip(req))
}

func (t *LoggingTransport) logResponse(resp *http.Response, err error) (*http.Response, error) {
	if err != nil {
		log.Printf("response error: %v", err)
		return resp, err
	}

	responseData := fmt.Sprintf("response: %v", resp)

	if resp.Body != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("reading response body: %v", err)
		}

		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		responseData = fmt.Sprintf("%s with body: %s", responseData, bodyBytes)
	}

	log.Println(responseData)

	return resp, err
}

func (t *LoggingTransport) logRequest(req *http.Request) *LoggingTransport {
	requestData := fmt.Sprintf("%s %s", req.Method, req.URL)

	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			log.Printf("reading request body: %v", err)
		}

		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		requestData = fmt.Sprintf("%s with body: %s", requestData, bodyBytes)
	}

	log.Println(requestData)

	return t
}

var _ http.RoundTripper = &LoggingTransport{}

func NewLoggingHttpClient() *http.Client {
	return &http.Client{
		Transport: &LoggingTransport{Transport: http.DefaultTransport},
	}
}
