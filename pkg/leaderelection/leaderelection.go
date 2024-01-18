package leaderelection

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func IsLeader() (bool, error) {
	electorPath := os.Getenv("ELECTOR_PATH")
	if electorPath == "" {
		// local development
		return true, nil
	}

	hostname, err := os.Hostname()
	if err != nil {
		return false, err
	}

	leader, err := getLeader(electorPath)
	if err != nil {
		return false, err
	}

	return hostname == leader, nil
}

func getLeader(electorPath string) (string, error) {
	const numRetries = 3

	resp, err := electorRequestWithRetry(electorPath, numRetries)
	if err != nil {
		return "", err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var electorResponse struct {
		Name string `json:"name"`
	}

	if err := json.Unmarshal(bodyBytes, &electorResponse); err != nil {
		return "", err
	}

	return electorResponse.Name, nil
}

func electorRequestWithRetry(electorPath string, numRetries int) (*http.Response, error) {
	client := http.Client{
		Timeout: time.Second * 5,
	}

	for i := 1; i <= numRetries; i++ {
		request, err := http.NewRequest(http.MethodGet, "http://"+electorPath, nil)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(request)
		if err == nil {
			return resp, nil
		}

		time.Sleep(time.Second * time.Duration(i))
	}

	return nil, fmt.Errorf("no response from elector container after %v retries", numRetries)
}
