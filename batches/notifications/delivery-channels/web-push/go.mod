module github.com/asari194617/taskly/batches/notifications/delivery-channels/web-push

go 1.25.4

require (
	github.com/SherClockHolmes/webpush-go v1.3.0
	github.com/asari194617/taskly/batches/notifications/delivery-channels/common v0.0.0
	github.com/go-sql-driver/mysql v1.10.0
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	golang.org/x/crypto v0.52.0 // indirect
)

replace github.com/asari194617/taskly/batches/notifications/delivery-channels/common => ../common
