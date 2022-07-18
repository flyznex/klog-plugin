package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// HandlerRegisterer is the symbol the plugin loader will try to load. It must implement the Registerer interface
var HandlerRegisterer = registerer("klog-plugin")

type registerer string

var logger Logger = nil

func (registerer) RegisterLogger(v interface{}) {
	l, ok := v.(Logger)
	if !ok {
		return
	}
	logger = l
	logger.Debug(fmt.Sprintf("[PLUGIN: %s] Logger loaded", HandlerRegisterer))
}

func (r registerer) RegisterHandlers(f func(
	name string,
	handler func(context.Context, map[string]interface{}, http.Handler) (http.Handler, error),
)) {
	f(string(r), r.registerHandlers)
}

func (r registerer) registerHandlers(ctx context.Context, extra map[string]interface{}, h http.Handler) (http.Handler, error) {
	cfg := configGetter(extra)
	logger.Info(fmt.Sprintf("[PLUGIN: %s] config for [skip_paths]: [%s]", HandlerRegisterer, cfg.listSkipPath()))

	// return the actual handler wrapping or your custom logic so it can be used as a replacement for the default http handler
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !cfg.Enabled {
			h.ServeHTTP(w, req)
			return
		}
		rpath := req.URL.Path
		record := map[string]interface{}{
			"method":     req.Method,
			"host":       req.Host,
			"path":       rpath,
			"user_agent": req.UserAgent(),
			"header":     req.Header,
			"query":      req.URL.RawQuery,
		}
		if !cfg.isSkipped(rpath) {
			br := req.Body
			var b []byte
			b, err := ioutil.ReadAll(br)
			if err != nil {
				logger.Error(fmt.Sprintf("[PLUGIN: %s] read request body error: %s", HandlerRegisterer, err.Error()))
			}
			record["request_body"] = string(b)
			// p, _ := json.Marshal(record)
			logger.Info("REQ: ", record)
			req.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		}
		loggingRW := &loggingResponseWriter{
			ResponseWriter: w,
		}
		h.ServeHTTP(loggingRW, req)
		record["response_status_code"] = loggingRW.status
		record["response_body"] = loggingRW.body
		record["response_headers"] = loggingRW.Header().Clone()
		p, _ := json.Marshal(record)
		logger.Info(string(p))
	}), nil
}

type Config struct {
	SkipPaths []string
	Enabled   bool
}

func configGetter(extra map[string]interface{}) Config {
	config, _ := extra[string(HandlerRegisterer)].(map[string]interface{})
	cfg := defaultConfigGetter()
	enable, ok := config["enabled"].(bool)
	if !ok {
		logger.Error(fmt.Sprintf("[PLUGIN: %s] config for [enabled] wrong!", HandlerRegisterer))
	}
	cfg.Enabled = enable
	skipPaths, ok := config["skip_paths"].([]interface{})
	if !ok {
		logger.Error(fmt.Sprintf("[PLUGIN: %s] config for [skip_paths] wrong!", HandlerRegisterer))
	}
	for _, v := range skipPaths {
		if p, ok := v.(string); ok {
			cfg.SkipPaths = append(cfg.SkipPaths, p)
		}
	}
	return cfg
}
func (c Config) isSkipped(path string) bool {
	isSkip := false
	for _, p := range c.SkipPaths {
		isSkip = path == p
		if isSkip {
			break
		}
	}
	return isSkip
}

func defaultConfigGetter() Config {
	return Config{
		SkipPaths: []string{},
		Enabled:   true,
	}
}

func (c Config) listSkipPath() string {
	return strings.Join(c.SkipPaths, ",")
}
func init() {
	fmt.Println("klog-plugin handler plugin loaded!!!")
}

func main() {}

type Logger interface {
	Debug(v ...interface{})
	Info(v ...interface{})
	Warning(v ...interface{})
	Error(v ...interface{})
	Critical(v ...interface{})
	Fatal(v ...interface{})
}

type loggingResponseWriter struct {
	status int
	body   string
	http.ResponseWriter
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *loggingResponseWriter) Write(body []byte) (int, error) {
	w.body = string(body)
	return w.ResponseWriter.Write(body)
}
