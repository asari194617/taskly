# GraphQL のキャンセルとリクエストの寿命

GraphQL は上流のキャンセルを引き継ぎ、5 秒の期限を設定する。resolver の DB・Redis・HTTP 操作には、このキャンセル可能なコンテキストを渡す。期限を過ぎた場合は HTTP 504、上流からキャンセルされた場合は既存のエラーハンドラーにキャンセルエラーを返す。

利用中の [graphql-go v0.8.1 の Execute](https://github.com/graphql-go/graphql/blob/v0.8.1/executor.go) は、キャンセル時に resolver の goroutine の終了を待たずに戻る。一方、Fiber のリクエストコンテキストはハンドラー終了時に再利用されるため、そのままでは実行中の resolver が解放後のリクエストを参照する。

このため、executor にはキャンセルで先に戻らないコンテキストを渡す。新しく構築したアプリケーションの schema の custom resolver は、起動時に一度だけラップし、各実行時に元のリクエストコンテキストを復元する。キャンセル後は次の custom resolver の処理を開始しない。ライブラリで共有される introspection の型や field は変更しない。

ハンドラーは、実行中の resolver がキャンセルに応じて終了してから応答する。5 秒は処理のキャンセル期限であり、終了処理まで含めた応答時間の上限ではない。新しい resolver の外部 I/O も `params.Context` に結び付け、期限のない待機やリクエストコンテキストを保持する goroutine を作らない。

`go test -race ./internal/handler/graphql/...` で、DB のキャンセル完了前に HTTP ハンドラーが戻らないこと、キャンセル後の mutation を開始しないこと、ネストした resolver と同時リクエストのコンテキストが正しく分離されることを確認する。
