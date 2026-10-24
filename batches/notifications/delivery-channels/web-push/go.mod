module github.com/asari194617/taskly/batches/notifications/delivery-channels/web-push

go 1.26.0

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/SherClockHolmes/webpush-go v1.4.0
	github.com/asari194617/taskly/batches/notifications/delivery-channels/common v0.0.0
	github.com/go-sql-driver/mysql v1.10.0
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	golang.org/x/crypto v0.58.0 // indirect
)

replace github.com/asari194617/taskly/batches/notifications/delivery-channels/common => ../common
