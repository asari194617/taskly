# メール送信者ロゴ（BIMI）

Tasklyから送るメールの送信者アイコンには、Brand Indicators for Message Identification（BIMI）を使用する。

## 公開ファイル

- ロゴ: `apps/web/public/bimi.svg`
- 本番URL: `https://taskly.iy0306.com/bimi.svg`

ロゴはSVG Tiny Portable/Secure（Tiny PS）形式、96 x 96 px、32 KB以下とする。外部参照、スクリプト、アニメーション、透過背景は使用しない。

## DNS設定

Webのデプロイ後、`taskly.iy0306.com` に次のTXTレコードを設定する。

```text
default._bimi.taskly.iy0306.com. TXT "v=BIMI1; l=https://taskly.iy0306.com/bimi.svg;"
```

BIMIを有効にする前提として、Fromドメインに整合するSPFまたはDKIMがすべての送信経路で成功し、DMARCは `p=quarantine` または `p=reject`、`pct=100` でなければならない。TasklyではResendのほか、残存しているSendGrid・Amazon SES用レコードも含め、実際に使用する経路を確認してからDMARCを強制する。

Gmailでロゴを表示するには、認証局が発行するVMCまたはCMCと、そのPEMファイルを参照するBIMIレコードが別途必要になる。証明書の取得前は、自己申告型BIMIに対応するメールボックスでのみ表示される可能性がある。

## リリース後の確認

1. `https://taskly.iy0306.com/bimi.svg` がHTTP 200、`image/svg+xml` で返ることを確認する。
2. `_dmarc.taskly.iy0306.com` が強制ポリシーかつ100%適用になっていることを確認する。
3. `default._bimi.taskly.iy0306.com` のTXTレコードとロゴURLを確認する。
4. SPF、DKIM、DMARCに合格したテストメールを対応メールボックスで確認する。
5. Gmailでの表示が必要な場合はVMCまたはCMCを取得し、PEM公開後にBIMIレコードへ `a=https://...pem` を追加する。
