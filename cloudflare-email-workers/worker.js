// 定数: 手動で編集してご利用ください
const ENDPOINT = "https://api.example.com/dmarc/inbound"; // dmarc-processor の公開URL + パス
const INTEGRATION_SECRET = "change_me_email_inbound_secret"; // サーバ側と一致させる

export default {
  async email(message, env, ctx) {
    try {
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

      const res = await fetch(ENDPOINT, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Email-Integration-Secret": INTEGRATION_SECRET,
        },
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        message.setRetry("forwarding failed");
        return;
      }
      // 処理を受け付けたらOK
      return;
    } catch (err) {
      message.setRetry("worker exception");
    }
  },
};
