module github.com/asari194617/taskly/batches/notifications/delivery-channels/discord

go 1.25.4

require (
	github.com/asari194617/taskly/batches/notifications/delivery-channels/common v0.0.0
	github.com/go-sql-driver/mysql v1.10.0
)

require filippo.io/edwards25519 v1.2.0 // indirect

replace github.com/asari194617/taskly/batches/notifications/delivery-channels/common => ../common
