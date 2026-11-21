package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type webshareProxy struct {
	Address  string `json:"proxy_address"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type webshareList struct {
	Results []webshareProxy `json:"results"`
}

// ResolveWebshare fetches a proxy list from the Webshare API and returns
// proxy URLs (http://user:pass@ip:port). Token comes from WEBSHARE_TOKEN.
func ResolveWebshare() ([]string, error) {
	token := os.Getenv("WEBSHARE_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("WEBSHARE_TOKEN not set")
	}
	url := "https://proxy.webshare.io/api/v2/proxy/list/?mode=direct&page=1&page_size=100"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Token "+token)
	c := &http.Client{Timeout: 30 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("webshare: %s", resp.Status)
	}
	var l webshareList
	if err := json.NewDecoder(resp.Body).Decode(&l); err != nil {
		return nil, err
	}
	out := []string{}
	for _, p := range l.Results {
		out = append(out, fmt.Sprintf("http://%s:%s@%s:%d", p.Username, p.Password, p.Address, p.Port))
	}
	return out, nil
}