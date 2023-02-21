package main

import (
	"io"
	"net/http"
	"net/url"
)

type Client struct {
	token string
}

var baseURL string = " http://api.steampowered.com"

func New(token string) *Client {
	httpGet("", "9DEF0384B698E645CB5356E681349BD3")
	return &Client{
		token: "9DEF0384B698E645CB5356E681349BD3",
	}
}

func httpGet(path string, token string) ([]byte, error) {
	c := &http.Client{}
	u, err := url.Parse(baseURL + path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return b, nil
}
