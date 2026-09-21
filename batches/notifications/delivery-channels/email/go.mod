module github.com/asari194617/taskly/batches/notifications/delivery-channels/email

go 1.25.4

require (
	github.com/asari194617/taskly/batches/notifications/delivery-channels/common v0.0.0
	github.com/go-sql-driver/mysql v1.10.0
	github.com/jhump/protoreflect v1.18.0
	google.golang.org/grpc v1.83.1
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/jhump/protoreflect/v2 v2.0.0-beta.2 // indirect
	github.com/petermattis/goid v0.0.0-20260701081913-4f67fd55d3b4 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/asari194617/taskly/batches/notifications/delivery-channels/common => ../common
