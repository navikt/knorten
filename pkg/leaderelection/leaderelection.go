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
	resp, err := electorRequestWithRetry(electorPath, 3)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var electorResponse struct {
		Name string
	}

	if err := json.Unmarshal(bodyBytes, &electorResponse); err != nil {
		return "", err
	}

	return electorResponse.Name, nil
}

func electorRequestWithRetry(electorPath string, numRetries int) (*http.Response, error) {
	for i := 1; i <= numRetries; i++ {
		resp, err := http.Get("http://" + electorPath)
		if err == nil {
			return resp, nil
		}
		time.Sleep(time.Second * time.Duration(i))
	}

	return nil, fmt.Errorf("no response from elector container after %v retries", numRetries)
}
