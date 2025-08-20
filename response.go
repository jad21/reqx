package reqx

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
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
		reader := r.resp.Body
		defer reader.Close()

		// Siempre leemos el cuerpo "tal cual" primero.
		raw, err := io.ReadAll(reader)
		if err != nil {
			r.readErr = fmt.Errorf("read raw body failed (status=%d, content-encoding=%q): %w",
				r.resp.StatusCode, r.resp.Header.Get("Content-Encoding"), err)
			return
		}

		// Si el Transport ya lo descomprimió, no intentes descomprimir otra vez.
		// (http.Transport lo hace cuando él mismo añadió "Accept-Encoding: gzip")
		if r.resp.Uncompressed {
			r.body = raw
			r.readErr = nil
			return
		}
		encodings := strings.TrimSpace(r.resp.Header.Get("Content-Encoding"))
		enc := strings.ToLower(encodings)
		if i := strings.Index(enc, ","); i >= 0 {
			enc = strings.TrimSpace(enc[:i])
		}

		switch enc {
		case "gzip":
			gr, err := gzip.NewReader(bytes.NewReader(raw))
			if err != nil {
				// Error con contexto
				r.readErr = fmt.Errorf("gzip.NewReader failed (status=%d, content-encoding=%q): %w",
					r.resp.StatusCode, encodings, err)
				// Si todo falla, deja crudo
				r.body = raw
				return
			}
			defer gr.Close()
			dec, derr := io.ReadAll(gr)
			if derr != nil {
				r.readErr = fmt.Errorf("read gzip body failed (status=%d, content-encoding=%q): %w",
					r.resp.StatusCode, encodings, derr)
				// Si todo falla, deja crudo
				r.body = raw
				return
			}
			r.body = dec
			r.readErr = nil

		case "deflate":
			if zr, err := zlib.NewReader(bytes.NewReader(raw)); err == nil {
				defer zr.Close()
				if dec, derr := io.ReadAll(zr); derr == nil {
					r.body = dec
					r.readErr = nil
					return
				} else {
					r.readErr = fmt.Errorf("read deflate(zlib) body failed (status=%d, content-encoding=%q): %w",
						r.resp.StatusCode, encodings, derr)
				}
			}
			// fallback a raw deflate
			fr := flate.NewReader(bytes.NewReader(raw))
			defer fr.Close()
			if dec, derr := io.ReadAll(fr); derr == nil {
				r.body = dec
				r.readErr = nil
				return
			}
			r.readErr = fmt.Errorf("read deflate(raw) body failed (status=%d, content-encoding=%q)", r.resp.StatusCode, encodings)
			// Si todo falla, deja crudo
			r.body = raw

		case "br":
			// Brotli no soportado nativamente
			dec, derr := io.ReadAll(bytes.NewReader(raw))
			if derr != nil {
				r.readErr = fmt.Errorf("read br body failed (status=%d, content-encoding=%q): %w",
					r.resp.StatusCode, encodings, derr)
				// Si todo falla, deja crudo
				r.body = raw
				return
			}
			r.body = dec
			r.readErr = nil

		default:
			// Sin codificación o desconocida -> crudo
			r.body = raw
			r.readErr = nil
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
