# Web セッション Cookie への移行（Issue #237）

Web クライアントは `api.taskly.iy0306.com` が設定するホスト専用の `taskly_session` Cookie で API に認証します。Cookie は `HttpOnly`、`SameSite=Lax` で、`NODE_ENV=production` の場合は `Secure` も指定します。API は不透明なトークンを Redis に 30 日間保存し、フォールバック検証用にハッシュを `sessions` に保存します。ネイティブクライアントは引き続き `Authorization: Bearer` を使い、ログイン応答でトークンを受け取ります。API キーと MCP トークンも従来どおりです。

本番環境では Web アプリに `https://taskly.iy0306.com`、API に `https://api.taskly.iy0306.com` を使います。オリジンは異なりますが同一サイトなので、Web クライアントは `credentials: "include"` を指定し、API は `API_CORS_ORIGINS` に明記したオリジンからのみ認証情報付きリクエストを許可します。Cookie の送信先は API ホストに限ります。オリジンのワイルドカードや `SameSite=None` の設定には、別途セキュリティレビューが必要です。

セッション Cookie を含むリクエストのうち、安全でない HTTP メソッドには、許可された `Origin` と `X-Client-Platform: web` の両方が必要です。このヘッダーは CORS のセーフリストに含まれていないため、Nginx と API の両方でプリフライト時に明示的に許可します。GraphQL のリクエスト（クエリも POST）と REST の更新も対象です。Bearer または `X-API-Key` だけで認証するリクエストにはこのヘッダーは不要です。本番でブラウザーからログインするには、許可された Origin が必要です。ブラウザーは `X-Client-Platform` を使ってもネイティブ向けの Bearer トークンを取得できません。

## リリース時の確認

1. 本番の `NODE_ENV=production`、`API_CORS_ORIGINS` に含まれる正確な Web オリジン（`https://taskly.iy0306.com`）、`CORS_ALLOW_CREDENTIALS=true` を確認します。Web ビルドの `PUBLIC_API_URL` が API の HTTPS オリジンを指すことを確認します。本番の `Secure` は必ず維持してください。
2. API と静的 Web クライアントのデプロイを調整します。ログイン応答の仕様が `token` から `authenticated` に同時に変わるため、バージョンが揃うまでログインに失敗することがあります。その時間帯を告知するか、ログインを一時停止し、リリース完了前に両方のデプロイを確認します。開いているページは再読み込みします。
3. 既存の Web セッションは再ログインが必要です。新しい Web バンドルは最初のリクエスト時に従来の `localStorage.api_token` を削除します。ブラウザー用の Bearer フォールバックやトークン交換エンドポイントはありません。ネイティブセッションは引き続き有効です。
4. パスワード、メールコード、TOTP・リカバリーコード、パスキーでのログインに加え、ページの再読み込み、GraphQL の更新、通知設定、ログアウト、Safari と PWA を簡易検証します。ログイン JSON にトークンが含まれず、応答の Cookie に `HttpOnly; Secure; SameSite=Lax` があることを確認します。無効な Origin や CSRF ヘッダーがないリクエストが拒否されることも確認します。
5. 必要なら Web と API の両方を直前のリリースへ同時に戻します。旧クライアントは HttpOnly Cookie を読めないため、Cookie で認証していたブラウザーでは再ログインが必要です。Cookie を localStorage にコピーしないでください。

オリジンやプロキシを変更する場合は、Cookie の Domain・Path、TLS 終端、CORS、`Origin` の検証、WebAuthn のリライングパーティーのオリジンを合わせて確認します。
