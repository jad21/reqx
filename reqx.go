package reqx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type multipartFile struct {
	FileName string
	Reader   io.Reader
}

var (
	globalClient     *http.Client
	globalClientOnce sync.Once
)

func getGlobalClient() *http.Client {
	globalClientOnce.Do(func() {
		globalClient = &http.Client{}
	})
	return globalClient
}

type RequestBuilder struct {
	ctx         context.Context
	method      string
	url         string
	params      url.Values
	headers     http.Header
	body        io.Reader
	isJSON      bool
	isMultipart bool
	files       map[string]multipartFile // fieldname => multipartFile
	formFields  map[string]string
	trace       *httptrace.ClientTrace // Para traza de cliente HTTP
}

// New inicializa el builder con un contexto base
func New() *RequestBuilder {
	return &RequestBuilder{
		ctx:        context.Background(),
		method:     "GET",
		params:     url.Values{},
		headers:    http.Header{},
		files:      make(map[string]multipartFile),
		formFields: make(map[string]string),
	}
}

// Method inicializa el builder con método y opcional URL, y contexto base
func Method(method string, urlStr ...string) *RequestBuilder {
	var u string
	if len(urlStr) > 0 {
		u = urlStr[0]
	}
	return &RequestBuilder{
		ctx:        context.Background(),
		method:     method,
		url:        u,
		params:     url.Values{},
		headers:    http.Header{},
		files:      make(map[string]multipartFile),
		formFields: make(map[string]string),
	}
}

// Métodos cortos para HTTP
func Get(urlStr ...string) *RequestBuilder    { return Method("GET", urlStr...) }
func Post(urlStr ...string) *RequestBuilder   { return Method("POST", urlStr...) }
func Put(urlStr ...string) *RequestBuilder    { return Method("PUT", urlStr...) }
func Delete(urlStr ...string) *RequestBuilder { return Method("DELETE", urlStr...) }
func Patch(urlStr ...string) *RequestBuilder  { return Method("PATCH", urlStr...) }

// WithContext permite cambiar el contexto
func (rb *RequestBuilder) WithContext(ctx context.Context) *RequestBuilder {
	rb.ctx = ctx
	return rb
}

// Otros setters
func (rb *RequestBuilder) Method(method string) *RequestBuilder { rb.method = method; return rb }
func (rb *RequestBuilder) URL(u string) *RequestBuilder         { rb.url = u; return rb }
func (rb *RequestBuilder) ParamsValues(values url.Values) *RequestBuilder {
	for k, vs := range values {
		for _, v := range vs {
			rb.params.Add(k, v)
		}
	}
	return rb
}
func (rb *RequestBuilder) Params(params map[string]string) *RequestBuilder {
	for k, v := range params {
		rb.params.Set(k, v)
	}
	return rb
}
func (rb *RequestBuilder) Param(key, value string) *RequestBuilder {
	rb.params.Set(key, value)
	return rb
}
func (rb *RequestBuilder) Header(key, value string) *RequestBuilder {
	rb.headers.Set(key, value)
	return rb
}
func (rb *RequestBuilder) Headers(pairs ...string) *RequestBuilder {
	if len(pairs)%2 != 0 {
		fmt.Println("Número de argumentos debe ser par")
		return rb
	}
	for i := 0; i < len(pairs); i += 2 {
		rb.headers.Set(strings.TrimSpace(pairs[i]), strings.TrimSpace(pairs[i+1]))
	}
	return rb
}
func (rb *RequestBuilder) IsJSON() *RequestBuilder { rb.isJSON = true; return rb }
func (rb *RequestBuilder) Bearer(token string) *RequestBuilder {
	rb.headers.Set("Authorization", "Bearer "+token)
	return rb
}
func (rb *RequestBuilder) Body(b []byte) *RequestBuilder {
	rb.body = bytes.NewReader(b)
	rb.isJSON = false
	rb.isMultipart = false
	return rb
}
func (rb *RequestBuilder) Json(data interface{}) *RequestBuilder {
	buf, err := json.Marshal(data)
	if err != nil {
		panic("Error serializando JSON: " + err.Error())
	}
	rb.body = bytes.NewReader(buf)
	rb.isJSON = true
	rb.isMultipart = false
	return rb
}
func (rb *RequestBuilder) Form(fields map[string]string) *RequestBuilder {
	for k, v := range fields {
		rb.formFields[k] = v
	}
	return rb
}
func (rb *RequestBuilder) File(fieldname, filepathStr string) *RequestBuilder {
	file, err := os.Open(filepathStr)
	if err != nil {
		panic("No se pudo abrir archivo: " + err.Error())
	}
	rb.files[fieldname] = multipartFile{FileName: filepath.Base(filepathStr), Reader: file}
	rb.isMultipart = true
	rb.isJSON = false
	return rb
}
func (rb *RequestBuilder) FileBytes(fieldname, filename string, data []byte) *RequestBuilder {
	rb.files[fieldname] = multipartFile{FileName: filename, Reader: bytes.NewReader(data)}
	rb.isMultipart = true
	rb.isJSON = false
	return rb
}
func (rb *RequestBuilder) FileReader(fieldname, filename string, reader io.Reader) *RequestBuilder {
	rb.files[fieldname] = multipartFile{FileName: filename, Reader: reader}
	rb.isMultipart = true
	rb.isJSON = false
	return rb
}

// WithTrace asocia un ClientTrace al RequestBuilder.
func (rb *RequestBuilder) WithTrace(trace *httptrace.ClientTrace) *RequestBuilder {
	rb.trace = trace
	return rb
}

// Do ejecuta la petición usando el contexto del builder, incluyendo traza si existe.
func (rb *RequestBuilder) Do(client ...*http.Client) (*Response, error) {
	u, _ := url.Parse(rb.url)
	if len(rb.params) > 0 {
		u.RawQuery = rb.params.Encode()
	}

	// Gestión de multipart, JSON y form fields...
	if rb.isMultipart && len(rb.files) > 0 {
		var bodyBuf bytes.Buffer
		writer := multipart.NewWriter(&bodyBuf)
		for field, mfile := range rb.files {
			part, err := writer.CreateFormFile(field, mfile.FileName)
			if err != nil {
				return nil, err
			}
			if _, err = io.Copy(part, mfile.Reader); err != nil {
				return nil, err
			}
			if f, ok := mfile.Reader.(*os.File); ok {
				f.Close()
			}
		}
		for k, v := range rb.formFields {
			_ = writer.WriteField(k, v)
		}
		writer.Close()
		rb.body = &bodyBuf
		rb.headers.Set("Content-Type", writer.FormDataContentType())
	} else if rb.isJSON {
		rb.headers.Set("Content-Type", "application/json")
	} else if len(rb.formFields) > 0 {
		form := url.Values{}
		for k, v := range rb.formFields {
			form.Set(k, v)
		}
		rb.body = strings.NewReader(form.Encode())
		rb.headers.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	// Crear la petición con el contexto original o envuelto con la traza
	reqCtx := rb.ctx
	if rb.trace != nil {
		reqCtx = httptrace.WithClientTrace(rb.ctx, rb.trace)
	}
	req, err := http.NewRequestWithContext(reqCtx, rb.method, u.String(), rb.body)
	if err != nil {
		return nil, err
	}
	for k, vals := range rb.headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	// Selección de cliente HTTP
	var c *http.Client
	if len(client) > 0 && client[0] != nil {
		c = client[0]
	} else {
		c = getGlobalClient()
	}
	// Ejecución de la petición
	r, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	return &Response{resp: r}, nil
}
