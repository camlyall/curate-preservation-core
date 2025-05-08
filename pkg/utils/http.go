package utils

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HttpClient struct {
	client *http.Client
}

// NewHttpClient creates a new HTTP client with the provided timeout and skipVerify settings.
func NewHttpClient(timeout time.Duration, skipVerify bool) *HttpClient {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipVerify},
	}
	return &HttpClient{
		client: &http.Client{
			Transport: tr,
			Timeout:   timeout,
		},
	}
}

func (c *HttpClient) Close() {
	c.client.CloseIdleConnections()
}

// DoRequest wraps the common HTTP request logic.
// It returns the full response for further handling.
func (c *HttpClient) DoRequest(ctx context.Context, method, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	// Create a new HTTP request with the provided context
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	// Set the provided headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Execute the HTTP request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	return resp, nil
}

// ParseResponse helps unmarshal JSON responses.
func ParseResponse(resp *http.Response, target any) error {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading body: %w", err)
	}
	return json.Unmarshal(body, target)
}

func Base64Encode(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

// Check connection to HTTP endpoint
// func CheckHTTPConnection(endpoint string) error {
// 	tr := &http.Transport{
// 		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
// 	}
// 	client := &http.Client{
// 		Transport: tr,
// 		Timeout:   5 * time.Second,
// 	}
// 	req, err := http.NewRequest("GET", endpoint, nil)
// 	if err != nil {
// 		return err
// 	}
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return err
// 	}
// 	defer resp.Body.Close()
// 	if resp.StatusCode != 200 {
// 		return fmt.Errorf("returned status code %d", resp.StatusCode)
// 	}
// 	return nil
// }
