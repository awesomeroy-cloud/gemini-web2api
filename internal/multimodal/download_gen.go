package multimodal

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"bytes"

	"github.com/ikhsan3adi/gemini-web2api/internal/gemini"
)

// DownloadGenerated fetches a gg-dl image with the account cookie.
// The first response is often a redirect onto usercontent.google.com.
func DownloadGenerated(client gemini.Requester, cookie string, rawURL string) ([]byte, string, error) {
	next, err := prepareMediaURL(rawURL)
	if err != nil {
		return nil, "", err
	}
	for hop := 0; hop < 5; hop++ {
		req, err := http.NewRequest(http.MethodGet, next, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		if cookie != "" && googleHost(req.URL.Hostname()) {
			req.Header.Set("Cookie", cookie)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, "", err
		}
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			resp.Body.Close()
			if loc == "" {
				return nil, "", fmt.Errorf("media redirect without location")
			}
			u, err := req.URL.Parse(loc)
			if err != nil {
				return nil, "", err
			}
			if !allowedMediaHost(u.Hostname()) {
				return nil, "", fmt.Errorf("media redirect host %s rejected", u.Hostname())
			}
			next = u.String()
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
		resp.Body.Close()
		if err != nil {
			return nil, "", err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("media download HTTP %d", resp.StatusCode)
		}
		mime := resp.Header.Get("Content-Type")
		if i := strings.Index(mime, ";"); i >= 0 {
			mime = mime[:i]
		}
		mime = strings.TrimSpace(mime)
		if mime == "" || mime == "text/plain" || bytes.HasPrefix(body, []byte("http")) {
			relay := strings.TrimSpace(string(body))
			if strings.HasPrefix(relay, "https://") {
				u, err := url.Parse(relay)
				if err != nil || !allowedMediaHost(u.Hostname()) {
					return nil, "", fmt.Errorf("media relay rejected")
				}
				next = relay
				continue
			}
		}
		return body, mime, nil
	}
	return nil, "", fmt.Errorf("media download exceeded redirects")
}

func prepareMediaURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || !allowedMediaHost(u.Hostname()) {
		return "", fmt.Errorf("unsupported media url")
	}
	if u.Hostname() == "lh3.googleusercontent.com" && strings.Contains(u.Path, "/gg-dl/") && !strings.Contains(u.Path, "=s") {
		u.Path += "=s1024-rj"
	}
	if strings.Contains(u.Path, "/gg-dl/") && u.Query().Get("alr") == "" {
		q := u.Query()
		q.Set("alr", "yes")
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

func allowedMediaHost(host string) bool {
	return host == "lh3.googleusercontent.com" || strings.HasSuffix(host, ".usercontent.google.com")
}

func googleHost(host string) bool {
	return host == "lh3.googleusercontent.com" || strings.HasSuffix(host, ".google.com") || strings.HasSuffix(host, ".googleusercontent.com") || strings.HasSuffix(host, ".usercontent.google.com")
}
