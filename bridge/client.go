package bridge

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"time"
)

type Client struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

func NewClient(baseURL, username, password string) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{
		Jar:       jar,
		Transport: tr,
		Timeout:   10 * time.Second,
	}

	return &Client{
		BaseURL:    baseURL,
		Username:   username,
		Password:   password,
		HTTPClient: client,
	}, nil
}

func (c *Client) Login() error {
	loginURL := fmt.Sprintf("%s/api/auth/login", c.BaseURL)
	payload := map[string]string{
		"username": c.Username,
		"password": c.Password,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Post(loginURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("login failed with status: %d", resp.StatusCode)
	}

	// Check login status to ensure session is active
	return c.CheckLoginStatus()
}

func (c *Client) CheckLoginStatus() error {
	statusURL := fmt.Sprintf("%s/api/miot/login_status", c.BaseURL)
	resp, err := c.HTTPClient.Get(statusURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("login status check failed with status: %d", resp.StatusCode)
	}
	return nil
}
