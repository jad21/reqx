package reqx

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
	method      string
	url         string
	params      url.Values
	headers     http.Header
	body        io.Reader
	isJSON      bool
	isMultipart bool
	files       map[string]multipartFile // fieldname => multipartFile
	formFields  map[string]string
}

// Métodos HTTP
func Method(method, urlStr string) *RequestBuilder {
	return &RequestBuilder{
		method:     method,
		url:        urlStr,
		params:     url.Values{},
		headers:    http.Header{},
		files:      make(map[string]multipartFile),
		formFields: make(map[string]string),
	}
}
func Get(urlStr string) *RequestBuilder    { return Method("GET", urlStr) }
func Post(urlStr string) *RequestBuilder   { return Method("POST", urlStr) }
func Put(urlStr string) *RequestBuilder    { return Method("PUT", urlStr) }
func Delete(urlStr string) *RequestBuilder { return Method("DELETE", urlStr) }
func Patch(urlStr string) *RequestBuilder  { return Method("PATCH", urlStr) }

// Parámetros de URL (varios)
func (rb *RequestBuilder) Params(params map[string]string) *RequestBuilder {
	for k, v := range params {
		rb.params.Set(k, v)
	}
	return rb
}

// Parámetro de URL (único)
func (rb *RequestBuilder) Param(key, value string) *RequestBuilder {
	rb.params.Set(key, value)
	return rb
}

// Header individual
func (rb *RequestBuilder) Header(key, value string) *RequestBuilder {
	rb.headers.Set(key, value)
	return rb
}

// Body RAW
func (rb *RequestBuilder) Body(b []byte) *RequestBuilder {
	rb.body = bytes.NewReader(b)
	rb.isJSON = false
	rb.isMultipart = false
	return rb
}

// JSON
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

// Campos de formulario
func (rb *RequestBuilder) Form(fields map[string]string) *RequestBuilder {
	for k, v := range fields {
		rb.formFields[k] = v
	}
	return rb
}

// Archivo para multipart (desde disco)
func (rb *RequestBuilder) File(fieldname, filepathStr string) *RequestBuilder {
	file, err := os.Open(filepathStr)
	if err != nil {
		panic("No se pudo abrir archivo: " + err.Error())
	}
	rb.files[fieldname] = multipartFile{
		FileName: filepath.Base(filepathStr),
		Reader:   file,
	}
	rb.isMultipart = true
	rb.isJSON = false
	return rb
}

// Archivo para multipart (desde memoria []byte)
func (rb *RequestBuilder) FileBytes(fieldname, filename string, data []byte) *RequestBuilder {
	rb.files[fieldname] = multipartFile{
		FileName: filename,
		Reader:   bytes.NewReader(data),
	}
	rb.isMultipart = true
	rb.isJSON = false
	return rb
}

// Archivo para multipart (desde io.Reader)
func (rb *RequestBuilder) FileReader(fieldname, filename string, reader io.Reader) *RequestBuilder {
	rb.files[fieldname] = multipartFile{
		FileName: filename,
		Reader:   reader,
	}
	rb.isMultipart = true
	rb.isJSON = false
	return rb
}

// Ejecución del request
func (rb *RequestBuilder) Do(client ...*http.Client) (*Response, error) {
	u, _ := url.Parse(rb.url)
	if len(rb.params) > 0 {
		u.RawQuery = rb.params.Encode()
	}

	// Si hay archivos: multipart/form-data
	if rb.isMultipart && len(rb.files) > 0 {
		bodyBuf := &bytes.Buffer{}
		writer := multipart.NewWriter(bodyBuf)

		// Archivos
		for field, mfile := range rb.files {
			part, err := writer.CreateFormFile(field, mfile.FileName)
			if err != nil {
				return nil, err
			}
			if _, err = io.Copy(part, mfile.Reader); err != nil {
				return nil, err
			}
			// Si el archivo viene de os.File, cerrarlo aquí
			if f, ok := mfile.Reader.(*os.File); ok {
				f.Close()
			}
		}

		// Otros campos de formulario
		for k, v := range rb.formFields {
			_ = writer.WriteField(k, v)
		}
		writer.Close()
		rb.body = bodyBuf
		rb.headers.Set("Content-Type", writer.FormDataContentType())
	} else if rb.isJSON {
		rb.headers.Set("Content-Type", "application/json")
	}

	req, err := http.NewRequest(rb.method, u.String(), rb.body)
	if err != nil {
		return nil, err
	}
	// Headers
	for k, v := range rb.headers {
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}

	var c *http.Client
	if len(client) > 0 && client[0] != nil {
		c = client[0]
	} else {
		c = getGlobalClient()
	}
	r, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	return &Response{resp: r}, nil
}

// DoCtx ejecuta la petición usando el context indicado y cliente opcional.
func (rb *RequestBuilder) DoCtx(ctx context.Context, client ...*http.Client) (*Response, error) {
	u, _ := url.Parse(rb.url)
	if len(rb.params) > 0 {
		u.RawQuery = rb.params.Encode()
	}

	// Si hay archivos: multipart/form-data
	if rb.isMultipart && len(rb.files) > 0 {
		bodyBuf := &bytes.Buffer{}
		writer := multipart.NewWriter(bodyBuf)

		// Archivos
		for field, mfile := range rb.files {
			part, err := writer.CreateFormFile(field, mfile.FileName)
			if err != nil {
				return nil, err
			}
			if _, err = io.Copy(part, mfile.Reader); err != nil {
				return nil, err
			}
			// Si el archivo viene de os.File, cerrarlo aquí
			if f, ok := mfile.Reader.(*os.File); ok {
				f.Close()
			}
		}

		// Otros campos de formulario
		for k, v := range rb.formFields {
			_ = writer.WriteField(k, v)
		}
		writer.Close()
		rb.body = bodyBuf
		rb.headers.Set("Content-Type", writer.FormDataContentType())
	} else if rb.isJSON {
		rb.headers.Set("Content-Type", "application/json")
	}

	// Petición con contexto
	req, err := http.NewRequestWithContext(ctx, rb.method, u.String(), rb.body)
	if err != nil {
		return nil, err
	}
	// Headers
	for k, v := range rb.headers {
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}

	var c *http.Client
	if len(client) > 0 && client[0] != nil {
		c = client[0]
	} else {
		c = getGlobalClient()
	}
	r, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	return &Response{resp: r}, nil
}
