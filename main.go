package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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

	config, _ := extra[string(HandlerRegisterer)].(map[string]interface{})
	enabled, ok := config["enabled"].(bool)
	if !ok {
		logger.Error(fmt.Sprintf("[PLUGIN: %s] config for [enabled] wrong!", HandlerRegisterer))
	}
	skipPaths, ok := config["skip_paths"].([]interface{})
	if !ok {
		logger.Error(fmt.Sprintf("[PLUGIN: %s] config for [skip_paths] wrong!", HandlerRegisterer))
	}
	logger.Info(fmt.Sprintf("[PLUGIN: %s] config for [skip_paths]: %v", HandlerRegisterer, skipPaths))

	// return the actual handler wrapping or your custom logic so it can be used as a replacement for the default http handler
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !enabled {
			h.ServeHTTP(w, req)
			return
		}
		rpath := req.URL.Path
		skip := false
		record := map[string]interface{}{
			"method":     req.Method,
			"host":       req.Host,
			"path":       rpath,
			"user_agent": req.UserAgent(),
			"header":     req.Header,
			"query":      req.URL.RawQuery,
		}
		for _, p := range skipPaths {
			if v, ok := p.(string); ok {
				skip = v == rpath
				if skip {
					break
				}
			}
		}
		if !skip {
			br := req.Body
			var b []byte
			b, err := ioutil.ReadAll(br)
			if err != nil {
				logger.Error(fmt.Sprintf("[PLUGIN: %s] read request body error: %s", HandlerRegisterer, err.Error()))
			}
			record["request_body"] = string(b)
			p, _ := json.Marshal(record)
			logger.Info(string(p))
			req.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		}
		h.ServeHTTP(w, req)
	}), nil
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
