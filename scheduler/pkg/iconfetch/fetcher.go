// Package iconfetch 提供从目标网站自动抓取 favicon 的能力。
// 设计为零业务依赖的独立模块，便于单元测试和在其他场景复用。
package iconfetch

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// 常量定义
const (
	// maxIconSize 允许下载的最大图标体积（字节）
	maxIconSize = 20 * 1024 // 20KB
	// requestTimeout 单次 HTTP 请求超时
	requestTimeout = 5 * time.Second
)

// FetchResult 抓取结果
type FetchResult struct {
	// IconData 图标 data URL，格式如 "data:image/svg+xml;base64,PHN2Zy..."
	IconData string
	// FetchedAt 实际抓取完成时间
	FetchedAt time.Time
	// Source 图标来源，取值 "link-tag" 或 "favicon-ico"
	Source string
}

// FetchError 抓取错误，携带底层原因
type FetchError struct {
	URL string
	Err error
}

func (e *FetchError) Error() string {
	return fmt.Sprintf("iconfetch: fetch %q failed: %v", e.URL, e.Err)
}

func (e *FetchError) Unwrap() error { return e.Err }

// 允许的图标 Content-Type 集合
var allowedMimes = map[string]bool{
	"image/svg+xml":            true,
	"image/png":                true,
	"image/jpeg":               true,
	"image/jpg":                true,
	"image/x-icon":             true,
	"image/vnd.microsoft.icon": true,
}

// 快捷模式 flag：使用 TLSClientConfig 跳过证书校验、短超时
var insecureTransport = &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	MaxIdleConns:    10,
	IdleConnTimeout: 30 * time.Second,
}

// linkRelIconRegex 匹配 <link rel="...icon..." ... href="...">
// 覆盖 rel="icon"、rel="shortcut icon"、rel="apple-touch-icon" 三种情况
var linkRelIconRegex = regexp.MustCompile(
	`<link[^>]+rel=["'](?:shortcut\s+)?(?:icon|apple-touch-icon)["'][^>]*href=["']([^"']+)["']`,
)

// linkHrefIconRegex 匹配 href 在前的变体：<link href="..." rel="...icon...">
var linkHrefIconRegex = regexp.MustCompile(
	`<link[^>]+href=["']([^"']+)["'][^>]*rel=["'](?:shortcut\s+)?(?:icon|apple-touch-icon)["']`,
)

// Fetch 从指定 URL 的域名下抓取 favicon。
// 返回 (FetchResult, nil) 表示成功；error 非 nil 时 result 的 IconData 为空字符串。
//
// 抓取逻辑（优先级从高到低）：
//  1. 请求 origin 根路径 HTML，用正则提取 <link rel="icon|shortcut icon|apple-touch-icon" href="...">
//  2. 直接请求 origin + "/favicon.ico"
//
// 任何一次 HTTP 请求都会跳过 TLS 证书校验并使用 5s 超时。
// 最终下载的图标必须 ≤ maxIconSize 且 mime 在允许列表内。
func Fetch(ctx context.Context, urlStr string) (FetchResult, error) {
	origin, err := extractOrigin(urlStr)
	if err != nil {
		return FetchResult{}, &FetchError{URL: urlStr, Err: err}
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var linkTagErr error // 保存 link-tag 路径的最后一个错误

	htmlBody, htmlErr := fetchHTML(ctx, origin)
	if htmlErr == nil {
		if iconHref := extractIconHref(htmlBody); iconHref != "" {
			absURL := resolveURL(origin, iconHref)
			icon, downloadErr := fetchIcon(ctx, absURL)
			if downloadErr == nil {
				return FetchResult{
					IconData:  icon.dataURL,
					FetchedAt: time.Now().UTC(),
					Source:    "link-tag",
				}, nil
			}
			linkTagErr = downloadErr
			slog.Debug("iconfetch: failed to download icon from link tag",
				"href", iconHref, "error", downloadErr)
		}
	} else {
		slog.Debug("iconfetch: failed to fetch HTML root",
			"origin", origin, "error", htmlErr)
	}

	// 回退：直接请求 /favicon.ico
	favURL := strings.TrimRight(origin, "/") + "/favicon.ico"
	icon, favErr := fetchIcon(ctx, favURL)
	if favErr == nil {
		return FetchResult{
			IconData:  icon.dataURL,
			FetchedAt: time.Now().UTC(),
			Source:    "favicon-ico",
		}, nil
	}

	return FetchResult{}, &FetchError{
		URL: urlStr,
		Err: fmt.Errorf("link-tag path: %v; favicon.ico path: %v",
			firstNonNil(linkTagErr, htmlErr), favErr),
	}
}

// firstNonNil 返回两个 error 中第一个非 nil 的
func firstNonNil(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// extractOrigin 从完整 URL 中提取 origin（scheme + "://" + host）
func extractOrigin(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("scheme must be http or https")
	}
	if u.Host == "" {
		return "", errors.New("missing host")
	}
	return u.Scheme + "://" + u.Host, nil
}

// fetchHTML GET 指定 origin 的根路径 HTML（不跟随外部重定向）
func fetchHTML(ctx context.Context, origin string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "BDopsFlow-IconFetcher/1.0")
	req.Header.Set("Accept", "text/html")

	client := &http.Client{
		Transport: insecureTransport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, 1024*1024) // HTML 最多 1MB
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// extractIconHref 在 HTML 源码中提取 favicon 的 href
func extractIconHref(html string) string {
	// rel 在 href 前
	if m := linkRelIconRegex.FindStringSubmatch(html); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	// href 在 rel 前
	if m := linkHrefIconRegex.FindStringSubmatch(html); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// resolveURL 将相对路径 href 拼成绝对 URL
func resolveURL(origin, href string) string {
	if strings.HasPrefix(href, "//") {
		// 协议相对 URL
		if strings.HasPrefix(origin, "https://") {
			return "https:" + href
		}
		return "http:" + href
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	// 相对路径
	origin = strings.TrimRight(origin, "/")
	if strings.HasPrefix(href, "/") {
		u, err := url.Parse(origin)
		if err != nil {
			return origin + href
		}
		return u.Scheme + "://" + u.Host + href
	}
	return origin + "/" + href
}

// iconData 下载到的图标字节 + 推断的 mime + 对应 data URL
type iconData struct {
	bytes   []byte
	mime    string
	dataURL string
}

// fetchIcon 下载指定 URL 的图标并校验格式/体积
func fetchIcon(ctx context.Context, iconURL string) (iconData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, iconURL, nil)
	if err != nil {
		return iconData{}, err
	}
	req.Header.Set("User-Agent", "BDopsFlow-IconFetcher/1.0")

	client := &http.Client{Transport: insecureTransport}
	resp, err := client.Do(req)
	if err != nil {
		return iconData{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return iconData{}, fmt.Errorf("icon url %q returned status %d", iconURL, resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxIconSize+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return iconData{}, err
	}
	if len(body) > maxIconSize {
		return iconData{}, fmt.Errorf("icon too large: %d bytes (limit %d)", len(body), maxIconSize)
	}

	// 先用 Content-Type，不行再用 magic number
	mime := detectMime(resp.Header.Get("Content-Type"), body)
	if !allowedMimes[mime] {
		return iconData{}, fmt.Errorf("icon mime %q not allowed", mime)
	}

	encoded := base64.StdEncoding.EncodeToString(body)
	return iconData{
		bytes:   body,
		mime:    mime,
		dataURL: "data:" + mime + ";base64," + encoded,
	}, nil
}

// detectMime 结合 Content-Type 和 magic number 判断真实 MIME
func detectMime(contentType string, body []byte) string {
	ct := strings.SplitN(strings.ToLower(strings.TrimSpace(contentType)), ";", 2)[0]
	ct = strings.TrimSpace(ct)
	if ct != "" && allowedMimes[ct] {
		return ct
	}

	// magic number 回退
	if len(body) >= 4 {
		switch {
		case body[0] == '<' && body[1] == '?' && body[2] == 'x' && body[3] == 'm':
			return "image/svg+xml"
		case body[0] == '<' && body[1] == 's' && body[2] == 'v' && body[3] == 'g':
			return "image/svg+xml"
		case body[0] == 0x89 && body[1] == 0x50 && body[2] == 0x4E && body[3] == 0x47:
			return "image/png"
		case body[0] == 0xFF && body[1] == 0xD8 && body[2] == 0xFF:
			return "image/jpeg"
		case body[0] == 0x00 && body[1] == 0x00 && body[2] == 0x01 && body[3] == 0x00:
			return "image/x-icon"
		}
	}
	if len(body) >= 2 && body[0] == '<' && body[1] == '?' {
		return "image/svg+xml"
	}

	return ct // 返回原始 Content-Type（可能为空），让上层 allowedMimes 统一判断
}
