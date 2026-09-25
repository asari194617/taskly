# 認証ユーザー作成・プロフィール同期

## 対象

GraphQL `adminUserCreate`, `adminUserBulkCreate`, `adminUserUpdate` のうち、利用者 ID、メール、表示名、通知言語、メールコード許可を認証サービスと同期する。認証 DB が ID とメール一意性の正本になり、アプリ DB は同じ ID の shadow 行を保持する。

## 作成

`AUTH_USERS_EXTERNAL=1` のとき、Taskly API は app DB へ先に insert せず、`POST {AUTH_USER_SYNC_URL}/internal/users` を呼ぶ。

1. 認証サービスが正規化メールの一意性を確認し、ID を採番する。
2. Taskly は返された ID を明示して app DB の `users` を作成する。パスワード、TOTP、復旧コード、パスキーは shadow 行へ新規保存しない。
3. 応答喪失や app insert 失敗後は同じメールで再送する。認証サービスは同じ ID を返す。
4. app DB に同じ ID とメールがすでにあれば再送成功とする。同じメールが異なる ID にある場合は自動上書きせず、同期保留として調査する。
5. CSV 一括作成は行ごとに同じ処理を行い、成功件数と行エラーを維持する。

フラグが `0` の開発環境と切り替え前は、従来どおり app DB が auto increment で採番する。

## 更新

管理者更新に認証同期対象の値が含まれる場合、先に `PATCH {AUTH_USER_SYNC_URL}/internal/users/{id}` を呼び、その後 app DB を更新する。

- 認証サービスでのメール競合や検証エラー時は app DB を変更しない。
- 認証更新後に app DB が失敗した場合は安全な認証状態を戻さず、同期保留を返す。同じ値の再送で収束させる。
- メール変更時は認証サービスが旧 session、challenge、code、OAuth token を失効する。Taskly の既存 session も対象利用者単位で削除して再認証を要求する。
- 初回ログイン時は認証サービスが `is_registered=1` を保存し、OAuth callback が同じ利用者 ID の app shadow 行も登録済みにする。
- REST `/users/me`、GraphQL `updateMe`、REST `/settings/lang` の本人更新も同じ internal API を先に呼び、認証 UI と OAuth userinfo の表示名・通知言語を app shadow と同期する。

## 設定と切り替え

```dotenv
AUTH_USERS_EXTERNAL=0
AUTH_USER_SYNC_URL=http://auth:3070
AUTH_INTERNAL_TOKEN=
```

- production で有効化する場合、URL は HTTPS または private network 上の HTTP、token は32文字以上とする。
- redirect は追従せず、認証サービスの応答本文やメールをエラーログへ出さない。
- 最終コピー、ID件数・ハッシュ、auto increment、作成・更新・再送テストが終わるまで `0` を維持する。

## 障害時

- 認証サービス呼び出し失敗は「同期保留」として返し、Discord / エラー監視には個人データを含めない。
- 認証側だけ作成済みなら、同じメールの再送で app shadow を作成する。手動 SQL で別 ID を採番しない。
- DB 復元後は最大 ID と auto increment、メールごとの ID 対応を両 DB で照合してから管理者作成を再開する。
