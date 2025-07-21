package reqx

import (
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	// "github.com/andybalholm/brotli" // Descomenta si usas brotli externo
)

type Response struct {
	resp    *http.Response
	body    []byte
	once    sync.Once
	readErr error
}

// Decodifica automáticamente según Content-Encoding
func (r *Response) cacheBody() {
	r.once.Do(func() {
		var reader io.ReadCloser = r.resp.Body
		defer reader.Close()

		encoding := strings.ToLower(strings.TrimSpace(r.resp.Header.Get("Content-Encoding")))

		switch encoding {
		case "gzip":
			gz, err := gzip.NewReader(reader)
			if err != nil {
				r.readErr = err
				return
			}
			defer gz.Close()
			r.body, r.readErr = io.ReadAll(gz)
		case "deflate":
			zr, err := zlib.NewReader(reader)
			if err != nil {
				r.readErr = err
				return
			}
			defer zr.Close()
			r.body, r.readErr = io.ReadAll(zr)
		case "br":
			// Brotli no soportado nativamente en Go estándar hasta 1.21.
			// Puedes usar la siguiente línea si importas github.com/andybalholm/brotli
			// br := brotli.NewReader(reader)
			// r.body, r.readErr = io.ReadAll(br)
			r.body, r.readErr = io.ReadAll(reader) // Por defecto, solo lee el stream
		default:
			r.body, r.readErr = io.ReadAll(reader)
		}
	})
}

func (r *Response) Bytes() ([]byte, error) {
	r.cacheBody()
	return r.body, r.readErr
}

func (r *Response) String() (string, error) {
	r.cacheBody()
	return string(r.body), r.readErr
}

func (r *Response) Json(v interface{}) error {
	r.cacheBody()
	if r.readErr != nil {
		return r.readErr
	}
	return json.Unmarshal(r.body, v)
}

func (r *Response) Status() int {
	return r.resp.StatusCode
}

func (r *Response) StatusText() string {
	return r.resp.Status
}

func (r *Response) Header() http.Header {
	return r.resp.Header
}

func (r *Response) Cookies() []*http.Cookie {
	return r.resp.Cookies()
}

func (r *Response) Location() (*url.URL, error) {
	return r.resp.Location()
}

func (r *Response) Raw() *http.Response {
	return r.resp
}

func (r *Response) ToFile(filename string) error {
	if r.readErr != nil {
		return r.readErr
	}
	return os.WriteFile("continue.html", r.body, 0644)
}

func (r *Response) IsOK() bool {
	return r.resp.StatusCode == http.StatusOK
}
