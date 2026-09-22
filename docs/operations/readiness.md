# API・MCP の readiness（Issue #396）

- `GET /healthz` はプロセスの生存確認です。依存先を検査しません。
- API `GET /readyz` は MySQL と Redis に ping し、2 秒以内に検査を終えます。どちらかが利用できなければ HTTP 503 を返します。
- MCP `GET /readyz` は MySQL の `SELECT 1` と Go API の `/readyz` を検査します。各検査は 1.5 秒、全体は 2 秒の上限を設け、失敗時は HTTP 503 を返します。認証トークンは不要です。
- Compose の API・MCP healthcheck とデプロイ後のホスト・公開経路確認は `/readyz` を使用します。Docker の `unhealthy` は依存先障害を表示し、この healthcheck 自体はコンテナを再起動しません。

障害調査ではまず `/healthz` と `/readyz` を両方確認します。前者のみ成功する場合は API で MySQL・Redis、MCP で MySQL・Go API の状態と通信経路を確認してください。復旧後に `/readyz` が HTTP 200 へ戻ることを確認します。レスポンス本文は依存先の詳細や接続情報を公開しません。
