package reqx

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestRequestBuilder_FormOnly(t *testing.T) {
	// Servidor de prueba que valida la petición entrante
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1) Content-Type
		ct := r.Header.Get("Content-Type")
		wantCT := "application/x-www-form-urlencoded"
		if ct != wantCT {
			t.Errorf("Content-Type = %q; quiero %q", ct, wantCT)
		}

		// 2) Cuerpo
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("error leyendo body: %v", err)
		}
		got := string(body)
		wantBody := "campo1=valor1&campo2=valor2"
		if got != wantBody {
			t.Errorf("Body = %q; quiero %q", got, wantBody)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Construcción de la petición con .Form
	_, err := Post(ts.URL).
		Form(map[string]string{
			"campo1": "valor1",
			"campo2": "valor2",
		}).
		Do()
	if err != nil {
		t.Fatalf("Do() devolvió error: %v", err)
	}
}
