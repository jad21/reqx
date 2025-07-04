# reqx

Librería ligera y tradicional para construir y ejecutar solicitudes HTTP en Go, con sintaxis encadenada (pipeline), soporte para JSON, multipart (archivos desde disco o memoria), cliente global o personalizado y cancelación con context.

---

## Características

- Todos los métodos HTTP (`GET`, `POST`, `PUT`, `DELETE`, `PATCH`)
- Encadenamiento: `.Params()`, `.Param()`, `.Header()`, `.Json()`, `.File()`, `.FileBytes()`, `.FileReader()`, `.Form()`
- Envío de archivos desde disco, memoria (`[]byte`) o cualquier `io.Reader`
- Cliente global por defecto o personalizado por petición
- Cancelación y deadlines usando `Context`
- Métodos para leer respuesta como `[]byte`, `string` o decodificar a `struct` (`Json`)
- Decodificación automática de cuerpos comprimidos (`gzip`, `deflate`)
- Helpers clásicos: acceso a headers, status, cookies, location, respuesta cruda, etc.
- Sin dependencias externas (brotli opcional)

---

Perfecto, gracias por el dato.
Aquí tienes la sección de **Instalación** correctamente ajustada para importar tu librería desde GitHub:

---

## Instalación

Puedes instalar la librería directamente con Go:

```bash
go get github.com/jad21/reqx
````

Y luego importar en tu código:

```go
import "github.com/jad21/reqx"
```

## Ejemplos de uso

### 1. **Solicitud GET simple con parámetros y header**

```go
import "github.com/jad21/reqx"

resp, err := reqx.
    Get("https://httpbin.org/get").
    Param("q", "valor").
    Header("Authorization", "Bearer TOKEN").
    Do()
if err != nil { /* manejar error */ }

// Leer body como string (decodifica gzip/deflate si corresponde)
body, err := resp.String()
fmt.Println(body)
````

---

### 2. **GET con varios parámetros**

```go
resp, err := reqx.
    Get("https://httpbin.org/get").
    Params(map[string]string{"q": "valor", "page": "1"}).
    Do()
if err != nil { /* manejar error */ }
fmt.Println(resp.Status(), resp.StatusText())
```

---

### 3. **POST enviando JSON y leyendo respuesta como struct**

```go
type MiRespuesta struct {
    Ok      bool   `json:"ok"`
    Mensaje string `json:"mensaje"`
}
payload := map[string]interface{}{"name": "Jad21", "activo": true}

resp, err := reqx.
    Post("https://api.ejemplo.com/crear").
    Header("Authorization", "Bearer TOKEN").
    Json(payload).
    Do()
if err != nil { /* manejar error */ }

var data MiRespuesta
if err := resp.Json(&data); err != nil {
    // manejo de error
}
fmt.Println(data.Ok, data.Mensaje)
```

---

### 4. **POST con archivo desde disco y campos de formulario**

```go
resp, err := reqx.
    Post("https://api.ejemplo.com/upload").
    File("documento", "/ruta/al/archivo.pdf").
    Form(map[string]string{
        "descripcion": "Mi archivo",
        "categoria":   "pdf",
    }).
    Do()
if err != nil { /* manejar error */ }
```

---

### 5. **POST con archivo en memoria (`[]byte`)**

```go
data := []byte("contenido de archivo en memoria")
resp, err := reqx.
    Post("https://api.ejemplo.com/upload").
    FileBytes("documento", "memoria.txt", data).
    Do()
if err != nil { /* manejar error */ }
```

---

### 6. **POST con archivo desde cualquier fuente (`io.Reader`)**

```go
var reader io.Reader = bytes.NewReader([]byte("contenido dinámico"))
resp, err := reqx.
    Post("https://api.ejemplo.com/upload").
    FileReader("documento", "dinamico.txt", reader).
    Do()
if err != nil { /* manejar error */ }
```

---

### 7. **Usar un `http.Client` personalizado**

```go
client := &http.Client{Timeout: 10 * time.Second}
resp, err := reqx.
    Get("https://httpbin.org/get").
    Do(client)
if err != nil { /* manejar error */ }
```

---

### 8. **Solicitudes con cancelación y timeout usando `Context`**

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

resp, err := reqx.
    Get("https://httpbin.org/delay/5").
    DoCtx(ctx) // Cancelará si demora más de 2 segundos
if err != nil {
    // err será context.DeadlineExceeded si ocurre timeout
    fmt.Println("Error:", err)
    return
}
body, _ := resp.String()
fmt.Println(body)
```

---

### 9. **Helpers de respuesta y acceso a métodos nativos**

```go
// Leer como []byte
b, err := resp.Bytes()

// Leer headers
fmt.Println(resp.Header().Get("Content-Type"))

// Acceder a cookies
for _, c := range resp.Cookies() {
    fmt.Println(c.Name, c.Value)
}

// Obtener la URL de redirección si existe
if loc, err := resp.Location(); err == nil {
    fmt.Println("Redirige a:", loc.String())
}

// Acceso directo a *http.Response (avanzado)
raw := resp.Raw()
fmt.Println("Protocolo:", raw.Proto)
```

---

### 10. **Decodificación automática de respuesta comprimida**

No necesitas hacer nada extra.
Si el servidor responde con `Content-Encoding: gzip` o `deflate`,
`resp.String()`, `resp.Bytes()` y `resp.Json()` lo manejan automáticamente.

---

## Notas

* Los archivos abiertos desde disco se cierran automáticamente.
* Puedes combinar archivos desde disco, memoria o streams en la misma petición.
* Brotli (`br`) requiere Go 1.21+ o un paquete externo para decodificación automática.
* Métodos `Do` y `DoCtx` coexisten para máxima compatibilidad y control.
* El wrapper `Response` expone helpers y métodos tradicionales del paquete estándar.

---

## Licencia

Uso libre, siguiendo buenas prácticas de la comunidad Go.

