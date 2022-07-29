package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// HandlerRegisterer is the symbol the plugin loader will try to load. It must implement the Registerer interface
var HandlerRegisterer = registerer("klog-plugin")
var tp *sdktrace.TracerProvider

var kfPusher *Pusher

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

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		if kfPusher != nil && cfg.KafkaConfig.Enabled {
			if err := kfPusher.Close(); err != nil {
				logger.Error(err)
				return
			}
			logger.Info(fmt.Sprintf("[PLUGIN: %s] Closed kafka connection", HandlerRegisterer))
		}
	}()

	// opentelemetry
	tp = sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	var tracer = otel.GetTracerProvider().Tracer(string(HandlerRegisterer))
	// return the actual handler wrapping or your custom logic so it can be used as a replacement for the default http handler
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !cfg.Enabled {
			h.ServeHTTP(w, req)
			return
		}
		rpath := req.URL.Path
		if skip := cfg.isSkipped(rpath); skip {
			h.ServeHTTP(w, req)
			return
		}
		if kfPusher == nil && cfg.KafkaConfig.Enabled {
			kfPusher = newPusher(ctx, cfg.KafkaConfig)
		}
		newCtx, span := tracer.Start(req.Context(), "Logging")
		span.SetAttributes(attribute.String("req.path", rpath))
		defer span.End()
		logRequest(cfg, newCtx, req, kfPusher)
		req = req.Clone(newCtx)
		loggingRW := &loggingResponseWriter{
			ResponseWriter: w,
		}
		span.AddEvent("REQUEST")
		h.ServeHTTP(loggingRW, req)
		span.AddEvent("RESPONSE")
		logResponse(cfg, newCtx, loggingRW, kfPusher)
	}), nil
}

func logRequest(cfg Config, ctx context.Context, req *http.Request, kfPusher ...*Pusher) {
	span := trace.SpanFromContext(ctx)
	record := map[string]interface{}{
		"method":     req.Method,
		"host":       req.Host,
		"path":       req.URL.Path,
		"user_agent": req.UserAgent(),
		"header":     req.Header,
		"query":      req.URL.RawQuery,
		"span_id":    span.SpanContext().SpanID().String(),
		"trace_id":   span.SpanContext().TraceID().String(),
	}
	for _, hk := range cfg.LogHeaderKeys {
		record[hk] = req.Header.Get(hk)
	}

	br := req.Body
	var b []byte
	b, err := ioutil.ReadAll(br)
	if err != nil {
		logger.Error(fmt.Sprintf("[PLUGIN: %s] read request body error: %s", HandlerRegisterer, err.Error()))
	}
	record["request_body"] = string(b)
	WriteLog("REQUEST", record, cfg.Stdout, kfPusher...)
	req.Body = ioutil.NopCloser(bytes.NewBuffer(b))
}

func logResponse(cfg Config, ctx context.Context, w *loggingResponseWriter, kfPusher ...*Pusher) {
	span := trace.SpanFromContext(ctx)
	rsprcd := map[string]interface{}{
		"span_id":              span.SpanContext().SpanID().String(),
		"trace_id":             span.SpanContext().TraceID().String(),
		"response_status_code": w.status,
		"response_body":        w.body,
		"response_headers":     w.Header().Clone(),
	}

	WriteLog("RESPONSE", rsprcd, cfg.Stdout, kfPusher...)
}

func WriteLog(prefix string, record map[string]interface{}, stdout bool, kfPusher ...*Pusher) {
	p, _ := json.Marshal(record)
	logEntry := string(p)
	if stdout {
		logger.Info(fmt.Sprintf("[PLUGIN: %s] %s: %s", HandlerRegisterer, prefix, logEntry))
	}
	for _, pusher := range kfPusher {
		if pusher != nil {
			logger.Info("push log to kafka")
			if err := pusher.Push(logEntry); err != nil {
				logger.Error(err)
			}
		}
	}
}

type Config struct {
	SkipPaths     []string
	Enabled       bool
	LogHeaderKeys []string
	Stdout        bool //enable print to stdout
	KafkaConfig   KafkaConfig
}
type KafkaConfig struct {
	Enabled   bool
	Brokers   []string
	Topic     string
	Partition int
}

func configGetter(extra map[string]interface{}) Config {
	config, _ := extra[string(HandlerRegisterer)].(map[string]interface{})
	cfg := defaultConfigGetter()
	enable, ok := config["enabled"].(bool)
	if !ok {
		logger.Warning(fmt.Sprintf("[PLUGIN: %s] config for [enabled] wrong!", HandlerRegisterer))
	}
	cfg.Enabled = enable
	skipPaths, ok := config["skip_paths"].([]interface{})
	if !ok {
		logger.Warning(fmt.Sprintf("[PLUGIN: %s] config for [skip_paths] not input or wrong!", HandlerRegisterer))
	}
	for _, v := range skipPaths {
		if p, ok := v.(string); ok {
			cfg.SkipPaths = append(cfg.SkipPaths, p)
		}
	}

	lHeader, ok := config["log_header_keys"].([]interface{})
	if !ok {
		logger.Warning(fmt.Sprintf("[PLUGIN: %s] config for [log_header_keys] not input or wrong!", HandlerRegisterer))
	}
	for _, v := range lHeader {
		if p, ok := v.(string); ok {
			cfg.LogHeaderKeys = append(cfg.LogHeaderKeys, p)
		}
	}
	kcfgRaw, ok := config["kafka"].(map[string]interface{})
	if !ok {
		logger.Info(fmt.Sprintf("[PLUGIN: %s] config for [kafka] is not config!", HandlerRegisterer))
	}
	kconfig := KafkaConfig{
		Enabled: false,
		Brokers: []string{},
		Topic:   "",
	}

	if ke, ok := kcfgRaw["enabled"].(bool); ok {
		kconfig.Enabled = ke
	}
	if urls, ok := kcfgRaw["broker_urls"].([]interface{}); ok {
		for _, u := range urls {
			if p, ok := u.(string); ok {
				kconfig.Brokers = append(kconfig.Brokers, p)
			}
		}
	}
	if tp, ok := kcfgRaw["topic"].(string); ok {
		kconfig.Topic = tp
	}
	cfg.KafkaConfig = kconfig

	if tp, ok := kcfgRaw["partition"].(int); ok {
		kconfig.Partition = tp
	}
	cfg.KafkaConfig = kconfig
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
		SkipPaths:     []string{},
		Enabled:       true,
		LogHeaderKeys: []string{},
		Stdout:        true,
		KafkaConfig:   KafkaConfig{Enabled: false},
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
