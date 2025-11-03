// 環境変数から設定を読み込みます（wrangler の Vars/Secrets を利用）
// - DMARC_INBOUND_ENDPOINT: dmarc-processor の公開URL + パス（例: https://api.example.com/dmarc/inbound）
// - INTEGRATION_SECRET: サーバ側と一致させる共有シークレット（Secret として設定推奨）

export default {
  async email(message, env, ctx) {
    try {
      const endpoint = env.DMARC_INBOUND_ENDPOINT;
      const integrationSecret = env.INTEGRATION_SECRET;

      if (!endpoint || !integrationSecret) {
        // 必須設定が無い場合はリトライしても意味がないので no-retry にするか、短時間での再試行を促す
        message.setNoRetry(
          "missing DMARC_INBOUND_ENDPOINT or INTEGRATION_SECRET"
        );
        return;
      }

      const from = message.from;
      const to = message.to;
      const subject = message.headers.get("Subject") || "";
      const headers = {};
      for (const [k, v] of message.headers) headers[k] = v;

      // RFC822 生データ(Base64) — サーバ側で解析する前提
      const rawArrayBuf = await new Response(message.raw).arrayBuffer();
      const rawB64 = btoa(String.fromCharCode(...new Uint8Array(rawArrayBuf)));

      const payload = {
        from,
        to,
        subject,
        headers,
        text: "",
        html: "",
        raw: rawB64,
        attachments: [],
      };

      const res = await fetch(endpoint, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Email-Integration-Secret": integrationSecret,
        },
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        // 一時的な失敗の可能性があるためリトライ
        message.setRetry(`forwarding failed: ${res.status}`);
        return;
      }
      // 処理を受け付けたらOK
      return;
    } catch (err) {
      message.setRetry("worker exception");
    }
  },
};
