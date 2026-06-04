package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func buildAPIURL(serverURL, path string, query map[string]string) (string, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}
	u.Path = path
	q := u.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func getJSON(url string, dest any) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	return json.Unmarshal(body, dest)
}
