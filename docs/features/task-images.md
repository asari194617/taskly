# 課題画像

課題の作成・編集時に JPEG、PNG、WebP、GIF を最大5枚添付できる。画像データは API コンテナを経由せず、ブラウザーから Cloudflare R2 に直接送信する。

## フロー

1. Web は画像の MIME type とサイズを `POST /task-images/presign` に送る。
2. API は最大サイズ（既定 10 MiB）と MIME type を検証し、`task-images/<user-id>/<uuid>` のキーで10分間有効な署名付き PUT URL を返す。
3. Web は返された署名対象ヘッダーを付け、R2 に直接 PUT する。
4. `createTask` / `updateTask` は `image_keys` を受け取り、R2 の `HeadObject` で存在、所有者メタデータ、MIME type、実サイズを再検証してから課題に関連付ける。
5. 課題一覧では15分間有効な署名付き GET URLだけを返す。R2のキーや公開URLはクライアントへ永続値として公開しない。

作成・編集フォームでは shadcn/ui の `Attachment` にファイル名と画像プレビューを表示する。作成・保存を押すと、署名付きURLの取得中とR2応答待ちは「処理中」、転送中は `XMLHttpRequest.upload` の実測値から割合とプログレスバーを表示する。転送量が取得できない場合は割合を付けない。R2の成功応答後に「アップロード完了」、失敗時は画像ごとにエラーを表示する。状態表示は日本語・英語に対応する。

送信中は入力、画像の追加・削除、保存の連打を無効にする。複数画像の一部が失敗しても全転送の終了を待ってからフォームを戻す。成功した画像のキーはフォーム内に保持し、再試行時は未送信・失敗画像だけをアップロードする。課題の保存に失敗した場合も送信済み画像を再利用する。画像の順番は選択順を維持する。

SVG は active content を含められるため許可しない。サイズ制限はクライアントとAPIの両方で検証し、最終的には課題保存時の `HeadObject` の実サイズで判定する。

課題カードの画像をクリックすると、ページ遷移せずダイアログでプレビューする。拡大・縮小（25〜400%、100%は画面内に収まる表示）、画面に合わせる、ダウンロード、画像コピー、新しいタブで開く操作に対応する。拡大画像はスクロールして全体を確認でき、Escape・閉じるボタン・背景クリックで閉じる。画像操作で課題の完了状態は変わらない。画像コピーはPNG形式に変換し、非対応ブラウザーや権限拒否・取得失敗を画面内で通知する。

ダウンロード・コピーは署名付きGET URLを `fetch` するため、R2のCORSには本番originからの **GET** 許可も必要。既存バケットのPUT許可を保持してGETを追加する。バケットを公開する必要はない。

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

Webの `apps/web/public/_headers` では、署名付きURLのバケットoriginを `connect-src`（アップロード）と `img-src`（表示）の両方で許可する。バケットやアカウントを変更する場合は、CSPと `apps/web/scripts/check-security-headers.mjs` の許可originも更新する。

R2 API token は対象バケットの Object Read & Write のみに限定する。バケットは非公開のままとし、Web の本番originからの直接 PUT と GET を許可するCORSポリシーを設定する。

```json
[
  {
    "AllowedOrigins": ["https://taskly.iy0306.com"],
    "AllowedMethods": ["PUT", "GET"],
    "AllowedHeaders": ["content-type", "x-amz-meta-owner-id"],
    "ExposeHeaders": ["etag"],
    "MaxAgeSeconds": 3600
  }
]
```

課題作成まで進まなかったアップロードは孤立オブジェクトになるため、DBの `tasks.image_key` と照合して未関連オブジェクトだけを削除する定期ジョブを運用する。課題の通常削除時はAPIが対応するR2オブジェクトを削除し、アカウント消去時はAirflowワーカーがユーザーprefixをまとめて削除する。DB移行は `0032_task_images.sql` を適用する。
