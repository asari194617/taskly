# 本番環境のネットワークポート

Taskly は、ホスト上の Nginx リバースプロキシとローカル監視が必要とするポートだけをホストに公開します。Docker サービス同士は Compose の `default` ネットワーク上で、サービス名を使って通信します。

## 公開範囲

| ホストポート | バインド先 | 用途 |
| --- | --- | --- |
| `22/tcp` | 外部公開。ホストとクラウドのファイアウォールで制限 | SSH による管理 |
| `80/tcp`、`443/tcp` | 外部公開 | Nginx の HTTP / HTTPS 入口 |
| `3063/tcp` | `127.0.0.1` のみ | Nginx の API 転送先 |
| `3064/tcp` | `127.0.0.1` のみ | ホスト内でのメトリクス取得 |
| `3065/tcp` | `127.0.0.1` のみ | Nginx の Airflow 転送先 |
| `3066/tcp` | `127.0.0.1` のみ | Nginx の MCP 転送先 |
| `3067/tcp` | `127.0.0.1` のみ | SSH トンネル経由の Grafana |
| `3068/tcp` | `127.0.0.1` のみ | Prometheus のクエリと準備状態の確認 |
| `3069/tcp` | `127.0.0.1` のみ | Alertmanager の準備状態の確認とテスト通知 |
| `3070/tcp` | `127.0.0.1` のみ | 別サービスの taskly-auth API 転送先 |
| `3071/tcp` | `127.0.0.1` のみ | 別サービスの長岡高専企業ガイダンス Go API 転送先 |

認証の転送先は別の taskly-auth デプロイが管理します。Taskly の Compose ファイルはそのサービスやポートマッピングを作成しません。[長岡高専企業ガイダンス](../nnct-corp-guidance-development.md)の API にも専用の Compose 構成と DB があります。静的な Next.js UI は R2 から配信するため、UI 用のホストポートは不要です。

MySQL（`3306`）、Redis（`6379`）、email-compose の gRPC（`50051`）はホストにマッピングしません。コンテナからは `db:3306`、`redis:6379`、`email-compose:50051` で接続します。

ホストとクラウドのファイアウォールは、明示的に許可したもの以外の着信を既定で拒否します。`80/tcp` と `443/tcp` は外部から許可し、可能なら `22/tcp` は信頼できる管理元に制限します。`3060`〜`3071` は外部ネットワークから許可しないでください。

## 検証

デプロイの前に、展開後の Compose 構成とリポジトリのポート公開方針を確認します。

```bash
docker compose --file infra/docker-compose.yml --file infra/docker-compose.prod.yml config --quiet
infra/ops/test-port-exposure.sh
```

デプロイ後にホストのバインド先を確認します。

```bash
docker compose --file infra/docker-compose.yml --file infra/docker-compose.prod.yml ps
ss -lnt
```

`3063`〜`3071` の Docker マッピングは `127.0.0.1` で、MySQL、Redis、email-compose のホストマッピングはありません。本番ネットワークの外にあるマシンから、以前に公開していた範囲を確認します。

```bash
for port in 3060 3061 3063 3064 3065 3066 3067 3068 3069 3070 3071; do
  nc -z -w 2 <production-host> "$port" && echo "$port unexpectedly open"
done
```

最後に Nginx 経由で公開 HTTPS エンドポイントを確認します。ループバック専用の転送先ポートに直接接続して確認しないでください。

## Nginx のセキュリティ基準

公開リバースプロキシでは Nginx の安定版 1.30.7 以降、メインライン 1.31.9 以降、または後続リリース系列を使います。これらは 2026-11-18 に公開されたセキュリティ修正を含みます。基準を設けた時点で Ubuntu の配布パッケージには公表された問題が残っていたため、通常のパッケージ更新後もアップストリームのバージョンを確認してください。

保護された `main` の `production-nginx-maintenance` ワークフローで、インストール済みのバイナリとパッケージ候補を確認します。`upgrade` は基準を下回る候補を拒否し、設定を検証し、Nginx を停止せず再読み込みして、サービスが稼働し続けることを確認します。パスワードなしで sudo を使えないホストでは `inspect` が設定検証のスキップを報告するため、更新前後に対話的に `nginx -t` を実行します。
