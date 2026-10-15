# 課題画像

課題の作成・編集時に JPEG、PNG、WebP、GIF を最大5枚添付できる。画像データは API コンテナを経由せず、ブラウザーから Cloudflare R2 に直接送信する。

## フロー

1. Web は画像の MIME type とサイズを `POST /task-images/presign` に送る。
2. API は最大サイズ（既定 10 MiB）と MIME type を検証し、`task-images/<user-id>/<uuid>` のキーで10分間有効な署名付き PUT URL を返す。
3. Web は返された署名対象ヘッダーを付け、R2 に直接 PUT する。
4. `createTask` / `updateTask` は `image_keys` を受け取り、R2 の `HeadObject` で存在、所有者メタデータ、MIME type、実サイズを再検証してから課題に関連付ける。
5. 課題一覧では15分間有効な署名付き GET URLだけを返す。R2のキーや公開URLはクライアントへ永続値として公開しない。

SVG は active content を含められるため許可しない。サイズ制限はクライアントとAPIの両方で検証し、最終的には課題保存時の `HeadObject` の実サイズで判定する。

## 設定

API の環境変数に以下を設定する。

```dotenv
R2_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com
R2_BUCKET=taskly-task-images
R2_ACCESS_KEY_ID=...
R2_SECRET_ACCESS_KEY=...
TASK_IMAGE_MAX_BYTES=10485760
```

本番では GitHub の `production` environment に `TASK_IMAGES_R2_ENDPOINT`、`TASK_IMAGES_R2_BUCKET`、`TASK_IMAGES_R2_ACCESS_KEY_ID`、`TASK_IMAGES_R2_SECRET_ACCESS_KEY` を専用Secretsとして保存する。デプロイ時に `.github/scripts/prepare-prod-env.py` が4項目を既存の `PROD_ENV` に組み込み、同名の設定を置き換える。専用Secretsを使わず `PROD_ENV` に4つの `R2_*` 設定を含めてもよい。専用Secretsを使う場合は4項目すべてが必須となる。デプロイ前の `.github/scripts/require-prod-env.sh` が未設定・空値を検出して停止する。リポジトリの同名 Secrets は公開MCPクライアントの配信用であり、APIコンテナには自動で渡らない。課題画像には配信用CDNと別の非公開バケットを使用する。

R2 API token は対象バケットの Object Read & Write のみに限定する。バケットは非公開のままとし、Web の本番originからの直接 PUT を許可するCORSポリシーを設定する。

```json
[
  {
    "AllowedOrigins": ["https://taskly.iy0306.com"],
    "AllowedMethods": ["PUT"],
    "AllowedHeaders": ["content-type", "x-amz-meta-owner-id"],
    "ExposeHeaders": ["etag"],
    "MaxAgeSeconds": 3600
  }
]
```

課題作成まで進まなかったアップロードは孤立オブジェクトになるため、DBの `tasks.image_key` と照合して未関連オブジェクトだけを削除する定期ジョブを運用する。課題の通常削除時はAPIが対応するR2オブジェクトを削除し、アカウント消去時はAirflowワーカーがユーザーprefixをまとめて削除する。DB移行は `0032_task_images.sql` を適用する。
