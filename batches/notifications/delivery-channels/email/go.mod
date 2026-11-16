module github.com/asari194617/taskly/batches/notifications/delivery-channels/email

go 1.27.4

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/asari194617/taskly/batches/notifications/delivery-channels/common v0.0.0
	github.com/go-sql-driver/mysql v1.10.1
	github.com/jhump/protoreflect v1.19.0
	google.golang.org/grpc v1.85.0
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/jhump/protoreflect/v2 v2.0.0-beta.2 // indirect
	github.com/petermattis/goid v0.0.0-20260701081913-4f67fd55d3b4 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260817212433-ac3dfec99bb1 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/asari194617/taskly/batches/notifications/delivery-channels/common => ../common
