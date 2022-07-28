module github.com/flyznex/klog-plugin

go 1.17

require (
	github.com/segmentio/kafka-go v0.4.32
	go.opentelemetry.io/otel v1.8.0
	go.opentelemetry.io/otel/sdk v1.8.0
)

require (
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/klauspost/compress v1.14.2 // indirect
	github.com/pierrec/lz4/v4 v4.1.14 // indirect
	go.opentelemetry.io/otel/trace v1.8.0 // indirect
	golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e // indirect
)

replace github.com/klauspost/compress v1.14.2 => github.com/klauspost/compress v1.15.6

replace golang.org/x/crypto v0.0.0-20190506204251-e1dfcc566284 => golang.org/x/crypto v0.0.0-20220331220935-ae2d96664a29

replace golang.org/x/net v0.0.0-20190404232315-eb5bcb51f2a3 => golang.org/x/net v0.0.0-20220520000938-2e3eb7b945c2

replace golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e => golang.org/x/sys v0.0.0-20220330033206-e17cdc41300f

replace golang.org/x/text v0.3.0 => golang.org/x/text v0.3.7

replace github.com/google/go-cmp v0.5.8 => github.com/google/go-cmp v0.5.7

replace golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543 => golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
