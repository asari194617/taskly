# 課題通知
## 仕様
### 技術
- Node.js
- React Email

### メールのテンプレート
- ログイン時の認証コード
- ログイン通知
- 課題締切通知
- パスワードリセット
- ユーザー情報の変更通知
- トークン・iCal発行通知
- 不適切投稿の通知(管理者向け)

### ディレクトリ構成

```
apps/email-compose/
  src/
    components/
      table.tsx       # 汎用のテーブルコンポーネント
    locales/
      en/
        common.json   # アプリ名など、コンポーネント以外からも呼び出される情報
        emails.json   # コンポーネント内部でのみ利用する情報
      ja/
        common.json
        emails.json
    messages/
    proto/
    templates/
    types/
  Dockerfile
  package-lock.json
  package.json
  README.md
  tsconfig.json
```

### 詳細
- Node.jsのマイクロサービスを構築し、その中で"メールの構成だけ"を責務とします。
- メールの送信ロジックは呼び出し元のマイクロサービスで行います。
- gRPCを使って、Node.jsのマイクロサービスと他のマイクロサービスが通信します。
- gRPCのサーバーとしてのみ動作します。フォールバックはありません。
- SentryとDatadogを導入し、エラーとパフォーマンス、ログを監視します。

- 日本語と英語に対応し、言語の情報はgRPCの引数に含めます。