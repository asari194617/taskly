# GraphQL 管理操作の認可

管理系 resolver は query / mutation それぞれの `requireAdminActor(c, allowManager)` を使う。`allowManager` は呼び出し箇所で指定する。未認証は `unauthorized`、ロールがない場合は `forbidden`、ロール照会の DB エラーは `authorization: ...` を返す。

| 操作 | 許可ロール |
| --- | --- |
| 日直の管理 query / mutation | admin、manager |
| 科目の管理 query / mutation | admin、manager |
| `adminUpdateDayShiftIndex`、`adminSetDayShiftCurrentCycle` | admin、manager |
| `adminSetConfig` | admin のみ |
| ユーザー管理 mutation | admin、manager |

ロール照会に失敗した場合は後続の DB 更新を行わない。認可拒否と DB 障害を区別できるよう、resolver でロール照会エラーを握りつぶさない。

回帰確認は `cd apps/server && go test ./internal/handler/graphql/...` を実行する。代表的な query / mutation について未認証、一般ユーザー、manager、admin、ロール照会失敗を検証する。
