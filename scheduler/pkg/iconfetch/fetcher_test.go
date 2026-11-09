package iconfetch

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ----- 测试 SVG 图标（用内联 SVG 做假数据） -----

const sampleSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" fill="#f46800"/><text x="16" y="24" text-anchor="middle" fill="white" font-size="20" font-family="sans-serif" font-weight="bold">G</text></svg>`

// TestFetch_HTMLLinkTag_SVG HTML 含 <link rel="icon" href> 时正确提取并下载 SVG
func TestFetch_HTMLLinkTag_SVG(t *testing.T) {
	mux := http.NewServeMux()

	// HTML 页面里放 link rel="icon"
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><link rel="icon" href="/logo.svg" type="image/svg+xml"></head><body>test</body></html>`)
	})

	mux.HandleFunc("/logo.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprint(w, sampleSVG)
	})

	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := Fetch(ctx, srv.URL+"/some/path")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.Source != "link-tag" {
		t.Errorf("Source = %q, want link-tag", result.Source)
	}
	if !strings.HasPrefix(result.IconData, "data:image/svg+xml;base64,") {
		t.Errorf("IconData prefix mismatch: got %q...", result.IconData[:40])
	}
	if len(result.IconData) < 30 {
		t.Errorf("IconData too short: %d chars", len(result.IconData))
	}
}

// TestFetch_HTMLLinkTag_Relative href 是相对路径时正确拼 origin
func TestFetch_HTMLLinkTag_Relative(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		// 用 ./assets/favicon.png 这种相对路径
		fmt.Fprintf(w, `<html><head><link rel="icon" href="./assets/favicon.svg"></head></html>`)
	})
	mux.HandleFunc("/assets/favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprint(w, sampleSVG)
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := Fetch(ctx, srv.URL+"/app/index.html")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.Source != "link-tag" {
		t.Errorf("Source = %q, want link-tag", result.Source)
	}
	if !strings.HasPrefix(result.IconData, "data:image/svg+xml;base64,") {
		t.Errorf("IconData prefix mismatch")
	}
}

// TestFetch_HTMLLinkTag_HrefFirst 覆盖 href 在前的 <link> 变体
func TestFetch_HTMLLinkTag_HrefFirst(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head><link href="/assets/icon.svg" rel="shortcut icon"></head></html>`)
	})
	mux.HandleFunc("/assets/icon.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprint(w, sampleSVG)
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := Fetch(ctx, srv.URL+"/")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.Source != "link-tag" {
		t.Errorf("Source = %q, want link-tag", result.Source)
	}
}

// TestFetch_FavIconIco HTML 无 link tag 但 /favicon.ico 可用 → 回退成功
func TestFetch_FavIconIco(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head></head><body>no link tag here</body></html>`)
	})
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprint(w, sampleSVG)
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := Fetch(ctx, srv.URL+"/")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.Source != "favicon-ico" {
		t.Errorf("Source = %q, want favicon-ico", result.Source)
	}
}

// TestFetch_Both404 HTML 和 favicon.ico 都 404 → 返回 error
func TestFetch_Both404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := Fetch(ctx, srv.URL+"/")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestFetch_Timeout 目标服务器不响应 → 5s 超时返回 error
func TestFetch_Timeout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 故意让请求挂住，超过 requestTimeout
		time.Sleep(7 * time.Second)
		fmt.Fprint(w, sampleSVG)
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	_, err := Fetch(ctx, srv.URL+"/")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed > 8*time.Second {
		t.Errorf("timeout too long: %v", elapsed)
	}
}

// TestFetch_TooLarge 图标超过 20KB → 返回 error
func TestFetch_TooLarge(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head><link rel="icon" href="/big.svg"></head></html>`)
	})
	// 构造一个大于 20KB 的"图标"
	bigSVG := `<svg xmlns="http://www.w3.org/2000/svg">` + strings.Repeat(`<rect x="0" y="0" width="1" height="1" fill="red"/>`, 4000) + `</svg>`
	mux.HandleFunc("/big.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprint(w, bigSVG)
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := Fetch(ctx, srv.URL+"/")
	if err == nil {
		t.Fatal("expected error for oversized icon, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestFetch_BadURL URL 非法 → 早返回 error
func TestFetch_BadURL(t *testing.T) {
	ctx := context.Background()
	_, err := Fetch(ctx, "not-a-url")
	if err == nil {
		t.Fatal("expected error for bad URL, got nil")
	}
}

// TestFetch_NonHTTP 非 http/https scheme → error
func TestFetch_NonHTTP(t *testing.T) {
	ctx := context.Background()
	_, err := Fetch(ctx, "ftp://example.com/file")
	if err == nil {
		t.Fatal("expected error for non-http scheme, got nil")
	}
	if !strings.Contains(err.Error(), "http or https") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestFetch_SSLCertFail httptest TLS + InsecureSkipVerify → 仍然成功
func TestFetch_SSLCertFail(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head><link rel="icon" href="/favicon.svg"></head></html>`)
	})
	mux.HandleFunc("/favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprint(w, sampleSVG)
	})

	// httptest.NewTLSServer 使用自签证书
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := Fetch(ctx, srv.URL+"/")
	if err != nil {
		t.Fatalf("Fetch() should succeed with self-signed cert: %v", err)
	}
	if result.Source != "link-tag" {
		t.Errorf("Source = %q, want link-tag", result.Source)
	}
}

// TestFetch_Base64Format 确认返回的 IconData 是标准 data URL 格式
func TestFetch_Base64Format(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head><link rel="icon" href="/favicon.svg"></head></html>`)
	})
	mux.HandleFunc("/favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprint(w, sampleSVG)
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := Fetch(ctx, srv.URL+"/")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	// 格式校验：data:image/svg+xml;base64,PHN2Zy...
	parts := strings.SplitN(result.IconData, ",", 2)
	if len(parts) != 2 {
		t.Fatalf("IconData format wrong: %s", result.IconData[:60])
	}
	if !strings.HasPrefix(parts[0], "data:image/") {
		t.Errorf("mime prefix wrong: %s", parts[0])
	}
	// parts[0] 应该是 data:image/svg+xml;base64
	if !strings.HasSuffix(parts[0], ";base64") {
		t.Errorf("missing base64 suffix in prefix: %s", parts[0])
	}
	// base64 部分不能含换行或空格
	if strings.ContainsAny(parts[1], "\n\r ") {
		t.Errorf("base64 body contains whitespace: %s...", parts[1][:40])
	}
}

// TestFetch_AppleTouchIcon rel="apple-touch-icon" 也能匹配
func TestFetch_AppleTouchIcon(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head><link rel="apple-touch-icon" href="/apple.png" sizes="180x180"></head></html>`)
	})
	mux.HandleFunc("/apple.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		// 伪造一个 PNG 的 magic number + 一些字节
		fmt.Fprint(w, "\x89PNG\r\n\x1a\n\x00\x00")
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := Fetch(ctx, srv.URL+"/")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.Source != "link-tag" {
		t.Errorf("Source = %q, want link-tag", result.Source)
	}
}

// TestFetch_SlashOrigin 确保 origin 末尾有 / 时不会产生双斜杠问题
func TestFetch_SlashOrigin(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head></head></html>`)
	})
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprint(w, sampleSVG)
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := Fetch(ctx, srv.URL+"/")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.Source != "favicon-ico" {
		t.Errorf("Source = %q, want favicon-ico", result.Source)
	}
}

// TestFetch_BlockingIconURL 返回 text/html 而非图片 MIME → allowedMimes 拒绝
func TestFetch_BlockingIconURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head><link rel="icon" href="/fake.png"></head></html>`)
	})
	// 返回 HTML 内容但声称是 PNG
	mux.HandleFunc("/fake.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html>this is not an image</html>")
	})
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := Fetch(ctx, srv.URL+"/")
	if err == nil {
		t.Fatal("expected error for disallowed mime, got nil")
	}
}
