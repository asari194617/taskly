# アカウント情報の認証サービスへの移管

`taskly-auth` は `/profile` で表示名の設定と現在のメールアドレスの確認を提供する。メールアドレスと確認コードは URL に含めない。確認コードは10分で失効し、5回誤入力すると使えなくなる。

表示名の正本は auth DB の `users.display_name` とする。認証サービスが表示名を更新した後、`PATCH {TASKLY_PROFILE_SYNC_URL}/internal/auth/users/{id}/profile` で Taskly の shadow row に反映する。両サービスの `AUTH_INTERNAL_TOKEN` が一致する必要がある。同期失敗時は `profile_sync_pending` を返す。同じ表示名を再保存すると再試行できる。

## 切り替え

1. [ユーザー移行](auth-user-projection-sync.md)を完了し、auth DB と app DB のユーザー ID、メールアドレス、表示名を照合する。`AUTH_MIGRATION_READY=1` と `AUTH_USERS_EXTERNAL=1` になるまで切り替えない。`AUTH_USERS_EXTERNAL` を含む8つの `AUTH_*_EXTERNAL` 所有権フラグは全て同時に有効化する必要がある。
2. auth 側の `TASKLY_PROFILE_SYNC_URL` に Taskly API の内部 origin (`http://server:8080`) を設定し、`AUTH_INTERNAL_TOKEN` を両サービスで一致させる。認証 API にプロフィールの各ルートが配備されたことと、内部同期経路を確認する。
3. Taskly Web に `PUBLIC_AUTH_PROFILE_URL=https://auth.taskly.iy0306.com/profile` と `PUBLIC_AUTH_SECURITY_URL=https://auth.taskly.iy0306.com/security` を設定し、API で `AUTH_PROFILE_EXTERNAL=1` にする。旧 GraphQL `updateMe` と REST `PATCH /users/me` の表示名更新を拒否する。通知言語と退会処理は Taskly に残る。
4. 確認コード送信、期限、誤入力上限、確認状態、メールアドレス変更後の状態リセットを検証する。

切り戻す際は Web の URL 設定と `AUTH_PROFILE_EXTERNAL` を戻す。auth DB で変更された表示名が shadow row へ同期済みであることを確認する。
