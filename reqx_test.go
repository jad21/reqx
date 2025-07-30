package reqx

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
	"testing"
	"time"
)

type jsonTestStruct struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
}

func startTestServer(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()

	// GET echo, with params/headers
	mux.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		w.Header().Set("X-Test", "123")
		_, _ = w.Write([]byte("param: " + q))
	})

	// POST json
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var in jsonTestStruct
		_ = json.NewDecoder(r.Body).Decode(&in)
		in.OK = true
		b, _ := json.Marshal(in)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})

	// POST multipart (file + field)
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseMultipartForm(1 << 20)
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte("multipart error"))
			return
		}
		field := r.FormValue("descripcion")
		file, _, err := r.FormFile("archivo")
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte("file error"))
			return
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		w.Write([]byte(field + ":" + string(data)))
	})

	// gzip response
	mux.HandleFunc("/gzip", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		gz.Write([]byte("gzip-content"))
	})

	// slow response for context cancel
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.Write([]byte("late"))
	})

	return httptest.NewServer(mux)
}

func TestGETWithParamAndHeader(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp, err := Get(ts.URL+"/get").
		Param("q", "valor").
		Header("X-Custom", "val").
		Do()
	if err != nil {
		t.Fatal(err)
	}
	txt, err := resp.String()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(txt, "param: valor") {
		t.Errorf("unexpected body: %s", txt)
	}
	if resp.Header().Get("X-Test") != "123" {
		t.Errorf("header not present")
	}
}

func TestPOSTJson(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	payload := jsonTestStruct{Name: "jad"}
	resp, err := Post(ts.URL + "/json").
		Json(payload).
		Do()
	if err != nil {
		t.Fatal(err)
	}
	var out jsonTestStruct
	if err := resp.Json(&out); err != nil {
		t.Fatal(err)
	}
	if !out.OK || out.Name != "jad" {
		t.Errorf("bad json response: %+v", out)
	}
}

func TestMultipartFileBytes(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	data := []byte("file-content")
	resp, err := Post(ts.URL+"/upload").
		FileBytes("archivo", "test.txt", data).
		Form(map[string]string{"descripcion": "test"}).
		Do()
	if err != nil {
		t.Fatal(err)
	}
	txt, _ := resp.String()
	if !strings.Contains(txt, "test:file-content") {
		t.Errorf("unexpected multipart response: %s", txt)
	}
}

func TestGzipResponse(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp, err := Get(ts.URL + "/gzip").Do()
	if err != nil {
		t.Fatal(err)
	}
	body, err := resp.String()
	if err != nil {
		t.Fatal(err)
	}
	if body != "gzip-content" {
		t.Errorf("gzip decode failed, got: %s", body)
	}
}

func TestContextCancel(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	start := time.Now()
	_, err := Get(ts.URL + "/slow").WithContext(ctx).Do()
	if err == nil {
		t.Errorf("expected timeout error")
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("context cancel took too long")
	}
}

func TestBytesAndStatusHelpers(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp, err := Get(ts.URL+"/get").Param("q", "a").Do()
	if err != nil {
		t.Fatal(err)
	}
	b, err := resp.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("param: a")) {
		t.Errorf("Bytes helper failed")
	}
	if resp.Status() != 200 {
		t.Errorf("expected status 200, got %d", resp.Status())
	}
	if !strings.HasPrefix(resp.StatusText(), "200") {
		t.Errorf("StatusText wrong: %s", resp.StatusText())
	}
}

// TestTrace valida que WithTrace inyecte callbacks en RequestBuilder
func TestTrace(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	// flags para verificar llamadas
	connectDone := false
	firstByte := false

	trace := &httptrace.ClientTrace{
		ConnectDone: func(network, addr string, err error) {
			connectDone = true
		},
		GotFirstResponseByte: func() {
			firstByte = true
		},
	}

	// Ejecutar petición mediante RequestBuilder con trace
	rb := Get(ts.URL + "/get").WithTrace(trace)
	_, err := rb.Do()
	if err != nil {
		t.Fatalf("error en Do(): %v", err)
	}
	// resp.Body.Close()

	if !connectDone {
		t.Error("esperaba ConnectDone invocado")
	}
	if !firstByte {
		t.Error("esperaba GotFirstResponseByte invocado")
	}
}
