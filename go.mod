module github.com/erniealice/hybra-golang

go 1.25.1

require github.com/erniealice/pyeza-golang v0.0.8-alpha

require (
	github.com/erniealice/esqyma v0.0.0
	github.com/google/uuid v1.6.0
)

require (
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251002232023-7c0ddcbb5797 // indirect
	google.golang.org/grpc v1.75.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/erniealice/esqyma => ../esqyma

replace github.com/erniealice/pyeza-golang => ../pyeza-golang
