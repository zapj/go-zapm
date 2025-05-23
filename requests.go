package zapm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HttpGetRequest 发送GET请求并返回响应体
func HttpGetRequest(urlStr string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "zapm")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// HttpPostRequest 发送POST请求并返回响应体
func HttpPostRequest(urlStr string, data interface{}) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	var bodyReader io.Reader

	if data != nil {
		bodyBytes, err := json.Marshal(data)
		if err != nil {
			return "", err
		}
		bodyReader = bytes.NewBuffer(bodyBytes)
	}

	req, err := http.NewRequest("POST", urlStr, bodyReader)
	if err != nil {
		return "", err
	}

	// 设置请求头
	req.Header.Set("User-Agent", "zapm")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("POST request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
