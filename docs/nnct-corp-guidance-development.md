# 長岡高専の企業ガイダンス

[企業ガイダンスのリポジトリ](https://github.com/asari194617/taskly-nnct-corp-guidance)では、企業検索、指定企業、スケジュール、会場マップ、「気になる」企業を扱います。認証には [taskly-auth](https://github.com/asari194617/taskly-auth) を使います。ガイダンスは独立したアプリケーションとして起動できます。

| サービス | 開発環境 | 本番環境 |
| --- | --- | --- |
| Next.js の画面 | `https://local.nnct-corp-guidance-2026.taskly.iy0306.com` | `https://nnct-corp-guidance-2026.taskly.iy0306.com` |
| Go API | `https://local.api.nnct-corp-guidance-2026.taskly.iy0306.com` | `https://api.nnct-corp-guidance-2026.taskly.iy0306.com` |
| MySQL | 専用の Compose DB。ホストからはループバックの `3308` | 本番 MySQL 上の専用データベースとDBアカウント |

Web クライアントは静的に書き出し、専用の Cloudflare R2 バケットで配信します。Go API はアプリケーションサーバーで動作し、開催情報、利用者ごとの「気になる」企業、名簿番号の設定を MySQL に保存します。

PDF と地図画像は別の非公開 R2 バケットで保管します。認証後に API が S3 署名付き URL を発行し、ブラウザは R2 から直接資料を取得します。API に資料用ボリュームはマウントしません。原本、抽出データ、認証情報は Git の外に置き、公開用バケットには静的な画面のみを配信します。

<a name="register-a-development-oauth-client"></a>

## 開発用 OAuth クライアントの登録

[共通のローカル HTTPS 手順](local-development.md#local-https)と[ガイダンスの HTTPS 設定](https://github.com/asari194617/taskly-nnct-corp-guidance/blob/main/docs/development.md#ローカル-https)に従って起動します。HTTPS を使う場合は、開発用の認証環境で `https://local.api.nnct-corp-guidance-2026.taskly.iy0306.com/auth/callback` を登録します。

ループバック HTTP を使う場合は、[taskly-auth の開発環境](https://github.com/asari194617/taskly-auth#開発環境)を起動して開発用ユーザーを用意し、`OAUTH_PUBLIC_CLIENTS_JSON` の配列に次の登録を追加します。他のクライアントの登録は保持し、同じ client ID の登録があれば更新してください。

```json
{"client_id":"nnct-corp-guidance-2026","redirect_uri":"http://127.0.0.1:3071/auth/callback"}
```

設定を変更したら、開発用の認証コンテナを再作成します。ガイダンスは PKCE S256 を使う公開クライアントです。Taskly の OAuth クライアントシークレットや内部同期用トークンは不要です。`taskly-web` と認証の所有権フラグは現在の設定を使います。Taskly と連携する開発環境では、ガイダンス専用の client ID `nnct-corp-guidance-2026` を使います。本番では別の認証環境に登録した HTTPS のコールバックを使います。

ループバック HTTP の画面・API・コールバックは `127.0.0.1` に揃えます。taskly-auth の公開クライアントで HTTP コールバックを使う場合、ホストにはループバック IP が必要です。ガイダンス API に送信するブラウザの `Origin` は、設定した画面のオリジンと完全に一致させます。

認証サービスのオリジンは独立して設定できます。ループバック HTTP では taskly-auth の既定の `localhost` を使用します。別の名前で起動している場合は、ガイダンスの `AUTH_ISSUER` にその実際の issuer を設定します。

## DB を起動して非公開データを取り込む

ガイダンスリポジトリでは Node.js 26.13.1（`.nvmrc`）と Go 1.27.4（`go.mod`）を使います。Taskly 自体の検証は Taskly のマニフェストとワークフローに従います。配布されたガイドブックの取り込みには Python 3.12、Pillow などのデータ抽出用依存関係、OCR 用の `tesseract` が必要です。macOS では `brew install tesseract`、Ubuntu では `sudo apt-get install tesseract-ocr` でインストールします。別のチェックアウトで以下を実行します。

```bash
git clone https://github.com/asari194617/taskly-nnct-corp-guidance.git
cd taskly-nnct-corp-guidance
docker compose up -d --wait db
python3 -m venv .venv
.venv/bin/python -m pip install -r scripts/requirements-data.txt
```

DB コンテナは新しい開発ボリュームを `schema.sql` で初期化します。Taskly と taskly-auth の DB から独立しています。既存ボリュームには明示的なスキーマ更新が必要です。Compose の環境変数を変えても、MySQL に保存済みの認証情報は変わりません。

既存のガイダンス DB では、名簿番号の設定に対応した API の起動前に `migrations/001_preferences.sql` を適用します。新規ボリュームでは `schema.sql` がテーブルを作成します。既存の**開発用** Compose DB に、レビュー済みの追加マイグレーションを適用するには以下を実行します。

```bash
docker compose exec -T db sh -c \
  'MYSQL_PWD="$MYSQL_PASSWORD" exec mysql -u"$MYSQL_USER" "$MYSQL_DATABASE"' \
  < migrations/001_preferences.sql
```

本番のマイグレーションは管理者が別途行います。アプリの DB アカウントに DDL 権限はなく、通常のイメージデプロイでもこのマイグレーションは適用しません。ガイダンスリポジトリのデプロイ手順に従ってください。

入力 PDF と非公開の出力先を選びます。出力先は**すべての Git チェックアウトの外**に置き、抽出前に例のパスを置き換えます。

```bash
export GUIDANCE_PDF='/absolute/path/to/guidebook.pdf'
export DATA_DIR='/absolute/private/path/to/guidance-data'
.venv/bin/python scripts/extract_guidance.py "$GUIDANCE_PDF" "$DATA_DIR"
export MYSQL_DSN='guidance:development-only@tcp(127.0.0.1:3308)/guidance?parseTime=true&charset=utf8mb4&loc=UTC'
go run ./cmd/import-guidance -check -input "$DATA_DIR/guidance.json"
go run ./cmd/import-guidance -input "$DATA_DIR/guidance.json"
```

例の DB 認証情報は、分離された開発用 Compose 環境専用です。抽出ツールは参照関係を検証し、API が使う非公開 JSON、マップ画像、資料を生成します。操作可能なブースの範囲は実際の PDF 上の矩形を使い、ブース番号を OCR で特定します。対応が曖昧な場合は抽出を中止します。抽出済み JSON と画像は一緒に更新します。ブースの範囲がない既存データで操作可能なマップを使うには、再抽出と再取り込みが必要です。出力、DB エクスポート、認証情報は両リポジトリ、フロントエンドのアセット、公開 CI アーティファクトに入れないでください。

インポーターはローカルのソースファイルを検証し、データセットと資料の許可リストをトランザクションで MySQL に書き込みます。資料の本体は R2 にアップロードしません。上記の `DATA_DIR` は管理者の非公開作業ディレクトリです。API の実行時設定やコンテナへのマウントには使いません。

アプリケーションの利用前に、非公開 JSON の `documents` 配列にあるファイル**だけ**を非公開の資料バケットにアップロードします。各 `file_name` をオブジェクトキーにします。抽出 JSON、名簿番号別の割り当てデータの単独エクスポート、DB エクスポートはどちらの R2 バケットにも入れません。アップロードには管理者用の書き込み認証情報を使います。アップロード後はオブジェクトを読み戻し、チェックサム、MIME タイプ、`Content-Disposition: inline`、`Cache-Control: no-store` を検証します。署名のないリクエストが拒否されることも確認します。アップロード、CORS、権限、ロールバックの詳細は、ガイダンスリポジトリの[非公開 R2 運用手順](https://github.com/asari194617/taskly-nnct-corp-guidance/blob/main/docs/private-r2.md)を参照してください。

## 非公開の資料ストレージを設定する

開発には非公開の開発用資料バケット、またはループバックで稼働中の S3 互換サービスを使います。API には R2 のエンドポイント、バケット、認証情報が必要で、ローカルファイルへのフォールバックはありません。署名には AWS SDK for Go v2、リージョン `auto`、パス形式のオブジェクト URL を使います。以下の例を開発用ストレージの設定に置き換えます。

```bash
export DOCUMENT_STORAGE='r2'
export R2_ENDPOINT='https://your-account-id.r2.cloudflarestorage.com'
export R2_BUCKET='your-private-development-documents'
export R2_ACCESS_KEY_ID='replace-with-bucket-scoped-read-only-access-key-id'
export R2_SECRET_ACCESS_KEY='replace-with-bucket-scoped-read-only-secret'
```

対象の非公開バケットに限定した、オブジェクト読み取り専用の認証情報を使います。公開 UI のデプロイキーや管理者のアップロードキーは流用しないでください。この設定は Go プロセスだけに渡し、`NEXT_PUBLIC_*`、フロントエンドビルド、Git、CI ログには含めません。ループバックで動作するオブジェクトサービスでは、ホストがループバックで API の開発用オリジンも HTTP の場合だけ、HTTP の `R2_ENDPOINT` を指定できます。サービスとバケットは、同じ署名方式と開発用の認証情報を受け付けるよう設定します。

非公開 R2 運用手順に従い、資料バケットの GET・HEAD の CORS ポリシーで開発用 UI オリジン `http://127.0.0.1:3000` を許可します。バケットは公開カスタムドメインも `r2.dev` アクセスも設定せず、非公開を維持します。GET と HEAD の署名は異なるため、署名済みの GET URL は HEAD リクエストに流用できません。

## API と Web クライアントを起動する

取り込みと同じターミナルで API を設定して起動します。

```bash
export ADDR='127.0.0.1:3071'
export UI_ORIGIN='http://127.0.0.1:3000'
export API_ORIGIN='http://127.0.0.1:3071'
export AUTH_ISSUER='http://localhost:3067'
guidance_registration='nnct-corp-guidance-2026'
export AUTH_CLIENT_ID="$guidance_registration"
export SESSION_ENCRYPTION_KEY="$(openssl rand -base64 32)"
go run ./cmd/api
```

セッションを再起動後も維持する場合は、生成した暗号鍵を非公開の開発設定に保存します。API はプロセスの環境変数を読み取ります。`.env.example` をコピーするだけでは `go run` に設定が読み込まれません。DSN にはシェルの特殊文字が含まれるため、エクスポート時の引用符を維持します。

別のターミナルで、ガイダンスのチェックアウトから実行します。

```bash
npm --prefix apps/web ci
npm --prefix deploy/r2 ci
NEXT_PUBLIC_API_URL='http://127.0.0.1:3071' npm --prefix apps/web run dev
```

`http://127.0.0.1:3000` を開き、開発用認証サービスからログインします。`GET /healthz` は MySQL 接続を確認し、`GET /readyz` はデータセットの取り込み済み状態も確認します。未認証で `/api/guidance`、`/api/documents/{id}`、`/api/bookmarks` にアクセスすると `401` が返ることを確認します。「気になる」企業はログイン中の Taskly アカウントに紐付き、ガイダンス DB に保存されます。

静的ページは `/`（ホーム）、`/search/`、`/favorites/`、`/schedule/`、`/documents/`、`/map/` にあります。本番では Cloudflare が URL とクエリ文字列を保持したまま、各ページを対応するエクスポート済み `index.html` に内部転送します。これらは独立したページで、従来の `?view=` URL は互換用です。マップはヘッダーとモバイルのナビゲーションの間を埋めます。オンラインの PDF・マップは Go サーバーのファイルシステムを経由せず、非公開 R2 から直接取得します。

初回ログイン時はセットアップダイアログで Ec4 の名簿番号を設定するか、後で設定することを選びます。API は取り込み済みのガイドに割り当てがある番号を受け付け、Taskly アカウントごとに選択を保存します。ユーザーメニューまたは「気になる」ページから変更でき、端末間で共有します。この設定から在籍は推定せず、URL にも保存しません。

新規ログインのローカルセッションには 30 日の絶対期限があります。API は上流のアクセストークンとローテーションするリフレッシュトークンを暗号化し、この期限を延ばさずにアクセスを自動更新します。保護されたリクエストでは引き続き上流の ID を確認します。既存の短いセッションは次回ログインまで従来の期限を維持し、名簿番号の設定と「気になる」企業も保持されます。

資料リンクは認証後に発行し、発行時のセッション残存時間を上限として、最長 5 分間有効です。PDF・マップ URL は閲覧者の Cookie なしで開くことができ、R2 の署名の期限まで利用できます。ログアウトすると新しい URL は発行できませんが、R2 が発行済みの URL は即時に失効しません。この一時 URL を公開ログ、Issue コメント、テストアーティファクトに含めないでください。

## 会場でのオフライン閲覧

オンラインで利用した後、同じ端末で、最後に取得できた企業情報、スケジュール、指定訪問、お気に入り、ホームの情報、ダウンロード済みのマップ画像を最大 24 時間閲覧できます。保存データは確認済みのアカウントと名簿番号に紐付きます。アカウント切り替え、ログアウト、`401`、名簿番号変更で削除します。オフライン時は閲覧専用で、課題の完了、お気に入り、名簿番号を変更できません。再接続時は書き込みを再送せず、サーバーの状態を再取得します。

Service Worker がキャッシュするのは公開の静的ファイルだけです。非公開のスナップショットと地図画像は別途 IndexedDB に保存し、認証データ、署名付き URL、PDF はキャッシュしません。PDF の閲覧と外部フォームには接続が必要です。既存の非公開 R2 バケットは正確な UI オリジンからの GET・HEAD を CORS で許可します。オブジェクトを公開したり、短時間の署名付き URL を恒久リンクに変えたりせずにマップ画像を取得できます。

通常のガイダンスビルドはアセットマニフェスト付きの `/offline-worker.js` を生成します。既存の R2 デプロイ手順は CSP を適用した最終 HTML からこのファイルを再生成し、アセット、HTML、Worker の順でアップロードします。`deploy/r2` の依存関係を入れると、CSP の準備とデプロイテストで使う `parse5` が利用できます。通常のビルド・デプロイには既存の Cloudflare 設定を使い、別の Worker サービスやバケットは不要です。

ゾーンのブラウザー TTL の上書きは、R2 オブジェクトの `no-cache` に優先する場合があります。キャッシュルールは正確な UI ホストと `/offline-worker.js` のみに限定し、エッジキャッシュを無効にし、ブラウザー TTL を `respect_origin` にします。デプロイ後は公開 Worker の JavaScript MIME タイプ、`no-cache`、最終アップロードとバイト単位での一致を検証します。不変アセット用のポリシーとゾーン全体の TTL は維持します。

デプロイ後に公開 UI URL を確認します。有効期限、キャッシュの範囲、CSP、Worker の検証条件は、ガイダンスリポジトリの[オフラインガイド](https://github.com/asari194617/taskly-nnct-corp-guidance/blob/main/docs/offline.md)を参照してください。

## ガイダンスの静的解析とフォーマット

以下は**ガイダンスのチェックアウト**から実行します。npm スクリプトは `apps/web` にありますが、Biome 2.6.7、Ruff 0.17.5、`go vet`、`gofmt` を使ってガイダンスリポジトリ全体を検査します。

```bash
cd /absolute/path/to/taskly-nnct-corp-guidance
npm --prefix apps/web ci
npm --prefix deploy/r2 ci
python3 -m venv .venv
. .venv/bin/activate
python3 -m pip install -r requirements-lint.txt
npm --prefix apps/web run lint
npm --prefix apps/web run format:check
```

フォーマットを適用するには `npm --prefix apps/web run format` を実行し、静的解析とフォーマットチェックを再実行します。CI はすべての変更で両方を確認し、フォーマットの適用やコミットは行いません。対象のソース形式と除外は、ガイダンスの[コード品質ガイド](https://github.com/asari194617/taskly-nnct-corp-guidance/blob/main/docs/code-quality.md)を参照してください。これらのコマンドは Taskly の静的解析や CI の設定には影響しません。

## 検証と本番運用

単体テスト、実 MySQL の統合テスト、ブラウザー E2E テスト、Go・フロントエンド・Docker のビルドには、ガイダンスリポジトリの検証手順とワークフローを使います。PR では単体、統合、ブラウザーのカバレッジを個別にレポートします。ドキュメントのみの変更を含む全 PR で必須チェックが動きます。パスフィルターにより必須チェックの結果が報告されなくなる構成は使いません。Go アプリケーション、フロントエンド、インポーター、PDF 抽出の対象テストは最低 80% を要求します。実 MySQL の実行は `^TestMySQL` を選択し、`internal/server` のステートメントカバレッジも別途 80% 以上を要求します。配信対象の Service Worker の関数は Node.js で別に計測し、行、分岐、関数それぞれで最低 80% を要求します。Chromium のページ E2E のカバレッジは、単体、MySQL、Service Worker とは別に報告されます。

ブラウザー E2E 用の `node scripts/e2e-run.mjs` は、テスト用 OAuth、実際の Go API、使い捨て MySQL DB に加え、専用の S3 互換サービスをループバックで起動します。合成ファイルを使い、SigV4、有効期限、Range、HEAD を独立に検証します。実 R2 の認証情報や本番資料は不要です。使い捨て DB の変数、ブラウザーのインストール、実行手順はガイダンスの[テストガイド](https://github.com/asari194617/taskly-nnct-corp-guidance/blob/main/docs/testing.md)に従ってください。

ガイダンスのリポジトリは GitHub ホストランナーを使い、GitHub Actions の Environment を `production-ui` と `production-api` に分けます。公開 UI のアップロードと、非公開資料の読み取りでは、それぞれ該当バケットに限定した別の認証情報を使います。API のデプロイには、デプロイコマンドに限定した専用 SSH 鍵を使い、サーバーのホスト ID を固定します。実行時 DB の認証情報、セッション暗号鍵、非公開バケットの読み取り専用キーはアプリケーションサーバーに置きます。PDF・PNG オブジェクトは非公開 R2 に保管し、原本の抽出と取り込みの検証は管理者の非公開作業ディレクトリで行います。実行時の Compose 構成はファイルをマウントせず、`DATA_DIR` も読み取りません。

環境設定、データの取り込み、準備状態の確認、ロールバックはガイダンスリポジトリのデプロイ手順を参照してください。Taskly の Compose ファイルと `infra/ops/apply-schema.sh` は、この別サービスのデプロイやマイグレーションを行いません。

## Taskly の課題完了を同期する

任意の自社ガイダンス連携は、独立した完了フラグを保存しません。ガイダンスは Taskly の既存の `tasks_users_completions` を読み書きします。片方で完了を付けたり外したりすると、もう片方の次回読み取りに反映されます。ガイダンスのホームを開くだけで学級共通の課題が作成されることはありません。

ガイダンスで指定した時限の訪問アンケートを新たに完了すると、対応する指定訪問が未完了なら同時に完了します。自動完了の由来は正規の行に保存するため、再読み込み後やサービス再起動後も保持されます。同じイベントと割り当て範囲で、この自動連動したペアのどちらかを**ガイダンス側で**取り消すと、両方の完了を外します。以前に手動で完了した訪問や、その後どちらかのサービスで明示的に手動完了したものは保持します。完了済みのアンケートを再度完了にしても、手動で取り消した訪問は再完了しません。

Taskly 側の操作は、引き続き 1 件ずつの手動操作です。対象項目の自動連動を解除し、他方の完了は変えません。名簿番号・企業の割り当て範囲が変わる場合、ガイダンスは他方の完了を保持し、古い自動完了の由来を解除します。

まず通常の Taskly 管理手順で連携先の課題を作成するか、既存の対象課題を特定します。実際の課題 ID と取り込んだイベントデータは、Git の外の非公開運用設定に置きます。Taskly API には以下を設定します。

- `NNCT_GUIDANCE_EVENT_ID` は取り込んだイベントの正確な不透明な名前空間です。
- `NNCT_GUIDANCE_TASK_BINDINGS` は論理キーを既存の正の一意な Taskly 課題 ID に対応付ける JSON オブジェクトです。受け付けるキーは `assigned-visit-N`、`visit-survey-N`（`N` は 1〜99）、`overall-survey`、`sheet-submission` です。固定キー両方と番号付きの各グループから少なくとも 1 件が必要です。バインディングは最大 64 件です。ガイダンス API は、非公開の取り込み済みスケジュールと選択した名簿番号の割り当てに照らし、必要なキーを検証します。
- 既存の機密クライアント用 `AUTH_ISSUER`、`AUTH_CLIENT_ID`、`AUTH_CLIENT_SECRET` と既存の `WEB_PUBLIC_URL` も設定します。本番では HTTPS が必要です。サービス全体の追加の連携シークレットはありません。

下記はバインディング形式を示す**架空の例**です。全値を非公開の運用設定に置き換え、本番では例の ID を使わないでください。

```json
{"assigned-visit-2":101,"visit-survey-2":102,"overall-survey":103,"sheet-submission":104}
```

設定変更後は Taskly API を再起動します。有効な設定がなければ連携は `503` を返し、既存の Taskly ルートは動作を続けます。連携をロールバックするには、2 つの `NNCT_GUIDANCE_*` の値を削除して再起動します。既存の課題と正規のユーザー完了記録は保持されます。

### 訪問アンケートと指定訪問の連動を導入する

既存のイベント名前空間、バインディング、課題、フォームのリンクを再利用します。環境変数、代替課題、フォームの追加は不要です。

Taskly の正式なスキーマは `apps/server/schema.sql` です。正規の `tasks_users_completions` テーブルに、由来を記録する NULL 許容列 `auto_event_id`、`auto_source_task_id`、`auto_assignment_scope` を追加します。既存の行には由来を NULL とし、手動完了として扱います。`apps/server/migrations` の履歴ファイルやガイダンス DB のマイグレーションではなく、Taskly の Atlas 手順で適用してください。

[ローカル開発](local-development.md)の環境設定を使い、**Taskly リポジトリのルート**で、新しい Taskly API の起動前に Atlas の計画を適用します。

```bash
infra/ops/apply-schema.sh
docker compose -f infra/docker-compose.yml up -d --build --wait server
```

**連動リクエストを送るガイダンス API をデプロイする前に、Taskly のスキーマと API を準備してください。** 本番スキーマに列がなければ、別途[スキーマの保守手順](operations/no-ddl-code-release.md#スキーマの保守)を実施します。Taskly の本番ワークフローは API の更新前に確立済みのスキーマ基準を検証します。Taskly を先に検証し、ロールバック時は両サービスを調整して、追加した NULL 許容列を保持します。課題 ID、フォームのリンク、イベントデータ、本番設定は Git の外に置きます。

`GET /integrations/nnct-guidance/tasks` と `PUT /integrations/nnct-guidance/tasks/{key}` を呼び出すのはガイダンスのバックエンドだけです。ログイン中のユーザーの有効な taskly-auth アクセストークンを `Authorization: Bearer …` で転送し、正確な `X-Guidance-Event-ID` を送ります。ブラウザーにはこのトークン、機密クライアントの認証情報、ユーザー間の識別子を渡しません。Taskly は自身の機密クライアントで毎回イントロスペクションを行い、専用の自社クライアント `nnct-corp-guidance-2026`、対象者 `taskly-api`、信頼された発行者、スコープ `taskly:login`、有効なトークン、数値のユーザーサブジェクトを要求します。開発テストでは分離した合成認証環境に同じクライアント ID を使います。無関係な OAuth クライアントからは連携経由で書き込めません。

GET は `{event_id,tasks:[{key,task_id,completed,completion_disabled,taskly_url}]}` を返します。通常の PUT は `{completed:boolean}` を送り、更新された課題オブジェクトを受け取ります。ガイダンスの連動操作は `link_assigned_visit:true` と、検証済みの不透明な `assignment_scope` を付けます。応答は相手の課題を `related_tasks` に含めます。このモードではアンケートの完了・取り消し、または指定訪問の取り消しを行います。指定訪問を明示的に完了する場合は、1 件の手動操作です。これらのフィールドはバックエンド間のプロトコルで、運用設定やブラウザーから渡す課題 ID ではありません。連動操作では両方の課題を同一トランザクションで検証・更新します。

どちらのモードも認証したユーザー本人の完了と取り消しを扱い、応答には `Cache-Control: no-store` を設定します。削除・ブロック済みアカウント、現行ポリシーへの同意、課題のモデレーション、読み取り権限、完了の無効化を検証します。完了が無効な課題でも、Taskly の通常操作と同じく取り消しは可能です。イベント不一致や完了の無効化は `409`、未登録のキーは `404`、参照できない課題は `403` を返します。認証や DB に障害があれば処理を拒否し、未完了状態を推測して返しません。

PR のカバレッジコメントはガイダンス連携パッケージ全体を対象とし、最低 80% を要求します。合成 OAuth / SQLite テストは ID、管理主体、設定、障害時の処理を検証します。`api-empty-database` はすべての PR の変更で、MySQL に対し `TestMySQLGuidanceBridgeCanonicalRoundTrip` も実行します。このテストは実際の連携と Taskly の GraphQL 完了ルートを双方向で検証し、取り消し、再試行、完了の無効化も含みます。本番データや認証情報は不要です。

サービス横断のブラウザーテストでは `apps/server/cmd/guidance-e2e` が、合成データとプロセス内 Redis を使い、実際の Taskly の連携と GraphQL ルートを起動します。`TASKLY_GUIDANCE_E2E=1`、本番以外の `NODE_ENV`、リテラルのループバック `ADDR`、ループバックを指し DB 名が正確に `taskly_guidance_e2e` の `TASKLY_E2E_MYSQL_DSN`、HTTP のループバック `AUTH_ISSUER` が必要です。データのある DB は削除せず、起動を拒否します。`schema.sql` を読み込めるように `apps/server` から起動するか、`TASKLY_E2E_SCHEMA_PATH` を渡します。合成発行者の `AUTH_CLIENT_ID` と `AUTH_CLIENT_SECRET` を設定してください。`/__test/ready` は準備状態を報告し、`/__test/fixture` は合成ユーザー ID、ローカルセッション、課題バインディング、イベント名前空間を返します。これらのルートとセッションはループバックのテストコマンドにのみ存在し、本番の Docker エントリーポイントは `cmd/server` だけをビルドします。フィクスチャには、認可失敗を検証する読み取り権限を明示的に拒否されたユーザーも含みます。ブラウザーテストは Taskly 側の変更に実際の `/graphql/` の完了ミューテーションを使い、逆方向にガイダンスのプロキシを使います。

導入前に合成連携、実 MySQL、ブラウザーのケースを検証します。対象はペアの作成、ガイダンスで両方向からの取り消し、手動完了の保持、Taskly の単一項目操作と連動の解除、再試行、範囲の変更、ユーザーの分離、書き込み失敗時のロールバックです。実際の CI 結果を記録します。

## ChatGPT への課題アクセスの委任

ガイダンスの MCP バックエンドは、利用者が同意した有効な ChatGPT アクセストークンを taskly-auth で交換してから Taskly の課題連携を呼び出します。MCP アクセストークンは Taskly に転送しません。Taskly は既存の `AUTH_CLIENT_ID` と `AUTH_CLIENT_SECRET` を引き続きイントロスペクションで使います。MCP バックエンドには、ガイダンス API の実行環境に専用の機密クライアント認証情報があり、このリポジトリやブラウザーには含めません。

この任意のポリシーを使うには、Taskly API の実行環境で信頼された `API_PUBLIC_URL` オリジンを設定し、`NNCT_GUIDANCE_MCP_PARENT_RESOURCE` を taskly-auth に登録したガイダンス API の正確な `/mcp` リソースと一致させます。本番ではいずれも HTTPS が必要です。委任設定がないか無効な場合は委任トークンを拒否し、既存の `nnct-corp-guidance-2026` の Web トークンのポリシーは維持します。どちらのリソースもリクエストヘッダーから組み立てません。

交換後のトークンは、クライアントとアクターがともに `nnct-guidance-mcp-backend`、元のクライアントが `nnct-guidance-chatgpt`、元のリソースが `NNCT_GUIDANCE_MCP_PARENT_RESOURCE` と一致する必要があります。対象者は `API_PUBLIC_URL` に `/integrations/nnct-guidance/tasks` を付けた値と厳密に一致します。taskly-auth はトークンの有効期間を 60 秒以内に制限し、イントロスペクションのたびに親の認可とトークンファミリーを検証します。交換でリフレッシュトークンは発行しません。Taskly の連携でも、現在時刻より 60 秒を超えて先の有効期限は拒否します。

交換では所定の連携プレフィックスの下で 1 つの操作だけを許可します。`GET /tasks` は `guidance:tasks:read`、`PUT /tasks/:key` は `guidance:tasks:write` を使います。複数スコープを組み合わせたり、対象者にガイダンス MCP を指定したりすると拒否します。通常の Web 連携と同じ権限、同意、アカウントブロック、完了、アンケート連動のトランザクション規則を適用します。この認証情報で Taskly の一般向け REST・GraphQL API は利用できません。

常に実行する `pr-coverage` ジョブは連携の単体テストと使い捨て MySQL の統合テストのカバレッジを独立に計測し、どちらも最低 80% を要求します。MySQL のセレクター `^TestMySQLGuidance` は既存の原子的な完了処理や由来のテストとともに、委任トークンのテストも選択します。ローカルで実行する場合は、分離した合成テスト用 DB のみを使います。
