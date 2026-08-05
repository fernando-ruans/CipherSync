package main

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const maxFaviconBytes = 512 * 1024

var errNotFound = errors.New("favicon not found")

var faviconLinkRe = regexp.MustCompile(`(?is)<link[^>]+href\s*=\s*["']([^"']+)["'][^>]*rel\s*=\s*["']([^"']*icon[^"']*)["']|<link[^>]+rel\s*=\s*["']([^"']*icon[^"']*)["'][^>]*href\s*=\s*["']([^"']+)["']`)

func extractDomain(rawURL string) string {
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func (v *Vault) getFaviconCached(domain string) (string, bool) {
	if v.db == nil {
		return "", false
	}
	var data string
	err := v.db.QueryRow(`SELECT data FROM favicons WHERE domain = ?`, domain).Scan(&data)
	if err != nil || data == "" {
		return "", false
	}
	return data, true
}

func (v *Vault) setFaviconCache(domain, data string) {
	if v.db == nil {
		return
	}
	_, _ = v.db.Exec(`INSERT INTO favicons (domain, data, fetched_at) VALUES (?, ?, ?)
		ON CONFLICT(domain) DO UPDATE SET data = excluded.data, fetched_at = excluded.fetched_at`,
		domain, data, time.Now().Unix())
}

type faviconFetcher struct {
	mu    sync.Mutex
	inFly map[string]bool
}

func newFaviconFetcher() *faviconFetcher {
	return &faviconFetcher{inFly: map[string]bool{}}
}

var faviconPool = newFaviconFetcher()

func httpGetWithTimeout(u string) ([]byte, string, error) {
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, "", err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", errNotFound
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFaviconBytes))
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("Content-Type"), nil
}

func fetchFavicon(domain string) (string, error) {
	if body, mime, err := httpGetWithTimeout("https://" + domain + "/favicon.ico"); err == nil && len(body) > 0 {
		return dataURI(mime, body), nil
	}
	html, _, err := httpGetWithTimeout("https://" + domain + "/")
	if err != nil {
		return "", err
	}
	href := findIconHref(string(html))
	if href == "" {
		return "", errNotFound
	}
	iconURL := href
	if strings.HasPrefix(href, "//") {
		iconURL = "https:" + href
	} else if strings.HasPrefix(href, "/") {
		iconURL = "https://" + domain + href
	} else if !strings.Contains(href, "://") {
		iconURL = "https://" + domain + "/" + href
	}
	if body, mime, err := httpGetWithTimeout(iconURL); err == nil && len(body) > 0 {
		return dataURI(mime, body), nil
	}
	return "", errNotFound
}

func findIconHref(html string) string {
	matches := faviconLinkRe.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		if m[1] != "" && strings.Contains(strings.ToLower(m[2]), "icon") {
			return m[1]
		}
		if m[3] != "" && strings.Contains(strings.ToLower(m[4]), "icon") {
			return m[4]
		}
	}
	return ""
}

func sniffExt(body []byte) string {
	if len(body) >= 8 && string(body[:4]) == "\x89PNG" {
		return "png"
	}
	if len(body) >= 4 && body[0] == 0 && body[1] == 0 && body[2] == 1 && body[3] == 0 {
		return "x-icon"
	}
	if len(body) >= 3 && body[0] == 0xff && body[1] == 0xd8 {
		return "jpeg"
	}
	if len(body) >= 6 && string(body[:6]) == "GIF87a" || len(body) >= 6 && string(body[:6]) == "GIF89a" {
		return "gif"
	}
	if len(body) >= 12 && string(body[:4]) == "RIFF" && string(body[8:12]) == "WEBP" {
		return "webp"
	}
	if len(body) >= 5 && string(body[:5]) == "<?xml" || len(body) >= 4 && string(body[:4]) == "<svg" {
		return "svg+xml"
	}
	return ""
}

func dataURI(mime string, body []byte) string {
	ext := sniffExt(body)
	if ext == "" {
		switch {
		case strings.Contains(mime, "image/x-icon"), strings.Contains(mime, "image/vnd.microsoft.icon"):
			ext = "x-icon"
		case strings.Contains(mime, "image/svg"):
			ext = "svg+xml"
		case strings.Contains(mime, "image/webp"):
			ext = "webp"
		case strings.Contains(mime, "image/png"):
			ext = "png"
		case strings.Contains(mime, "image/jpeg"), strings.Contains(mime, "image/jpg"):
			ext = "jpeg"
		default:
			ext = "png"
		}
	}
	return "data:image/" + ext + ";base64," + base64.StdEncoding.EncodeToString(body)
}
