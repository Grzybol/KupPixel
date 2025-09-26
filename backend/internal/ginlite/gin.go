package gin

import (
	"encoding/json"
	"log"
	"mime"
	"net/http"
	"os"
	pathpkg "path/filepath"
	"strings"
	"time"
)

type HandlerFunc func(*Context)

type H map[string]interface{}

type Context struct {
	Writer   http.ResponseWriter
	Request  *http.Request
	params   map[string]string
	handlers []HandlerFunc
	index    int
	values   map[string]any
}

func (c *Context) JSON(status int, body interface{}) {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(c.Writer).Encode(body); err != nil {
		log.Printf("ginlite: JSON encode error: %v", err)
	}
}

func (c *Context) Status(status int) {
	c.Writer.WriteHeader(status)
}

func (c *Context) String(status int, body string) {
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.WriteHeader(status)
	_, _ = c.Writer.Write([]byte(body))
}

func (c *Context) ShouldBindJSON(out interface{}) error {
	decoder := json.NewDecoder(c.Request.Body)
	return decoder.Decode(out)
}

func (c *Context) Param(name string) string {
	return c.params[name]
}

func (c *Context) Set(key string, value any) {
	if c.values == nil {
		c.values = make(map[string]any)
	}
	c.values[key] = value
}

func (c *Context) Get(key string) (any, bool) {
	if c.values == nil {
		return nil, false
	}
	value, ok := c.values[key]
	return value, ok
}

func (c *Context) Next() {
	c.index++
	for c.index < len(c.handlers) {
		c.handlers[c.index](c)
		c.index++
	}
}

type route struct {
	method  string
	path    string
	handler HandlerFunc
}

type Engine struct {
	routes     []route
	noRoute    HandlerFunc
	middleware []HandlerFunc
}

func Default() *Engine {
	return &Engine{}
}

func (e *Engine) Use(handlers ...HandlerFunc) {
	e.middleware = append(e.middleware, handlers...)
}

func (e *Engine) addRoute(method, path string, handler HandlerFunc) {
	combined := make([]HandlerFunc, 0, len(e.middleware)+1)
	combined = append(combined, e.middleware...)
	combined = append(combined, handler)
	e.routes = append(e.routes, route{method: method, path: path, handler: func(c *Context) {
		c.handlers = combined
		c.index = -1
		c.Next()
	}})
}

func (e *Engine) GET(path string, handler HandlerFunc) {
	e.addRoute(http.MethodGet, path, handler)
}

func (e *Engine) POST(path string, handler HandlerFunc) {
	e.addRoute(http.MethodPost, path, handler)
}

func (e *Engine) Static(relativePath, root string) {
	fs := http.FileServer(http.Dir(root))
	e.GET(relativePath+"/*filepath", func(c *Context) {
		http.StripPrefix(relativePath, fs).ServeHTTP(c.Writer, c.Request)
	})
}

func (e *Engine) StaticFS(relativePath string, fs http.FileSystem) {
	handler := http.StripPrefix(relativePath, http.FileServer(fs))
	e.GET(relativePath+"/*filepath", func(c *Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	})
}

func (e *Engine) NoRoute(handler HandlerFunc) {
	e.noRoute = handler
}

func (e *Engine) Run(addr string) error {
	return http.ListenAndServe(addr, e)
}

func (e *Engine) match(method, path string) (HandlerFunc, map[string]string) {
	for _, r := range e.routes {
		if r.method != method {
			continue
		}
		if r.path == path {
			return r.handler, map[string]string{}
		}
		if strings.Contains(r.path, "*") {
			prefix := strings.Split(r.path, "*")[0]
			if strings.HasPrefix(path, prefix) {
				return r.handler, map[string]string{"filepath": strings.TrimPrefix(path, prefix)}
			}
		}
	}
	return nil, nil
}

func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler, params := e.match(r.Method, r.URL.Path)
	if handler == nil {
		if e.noRoute != nil {
			ctx := &Context{Writer: w, Request: r, params: map[string]string{}, index: -1}
			e.noRoute(ctx)
			return
		}
		http.NotFound(w, r)
		return
	}
	ctx := &Context{Writer: w, Request: r, params: params, index: -1}
	handler(ctx)
}

func ServeFile(c *Context, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		http.NotFound(c.Writer, c.Request)
		return
	}
	defer file.Close()

	ext := strings.ToLower(pathpkg.Ext(filePath))
	if mime := mime.TypeByExtension(ext); mime != "" {
		c.Writer.Header().Set("Content-Type", mime)
	}
	info, err := file.Stat()
	if err != nil {
		http.NotFound(c.Writer, c.Request)
		return
	}
	http.ServeContent(c.Writer, c.Request, filePath, info.ModTime(), file)
}

func StaticFileHandler(filePath string) HandlerFunc {
	return func(c *Context) {
		ServeFile(c, filePath)
	}
}

// Convenience helper for simple HTML responses
func HTML(c *Context, status int, body string) {
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Writer.WriteHeader(status)
	_, _ = c.Writer.Write([]byte(body))
}

// AddTimeout wraps a handler with a simple timeout.
func AddTimeout(h HandlerFunc, d time.Duration) HandlerFunc {
	return func(c *Context) {
		done := make(chan struct{})
		go func() {
			h(c)
			close(done)
		}()
		select {
		case <-done:
			return
		case <-time.After(d):
			c.Writer.WriteHeader(http.StatusGatewayTimeout)
		}
	}
}
