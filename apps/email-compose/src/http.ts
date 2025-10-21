import http from "node:http";
import { render } from "@react-email/render";
import { LoginCodeEmail } from "./templates/LoginCode.js";
import { t as translate } from "./i18n.js";

export function startHttpServer(port: number) {
  const server = http.createServer(async (req, res) => {
    try {
      if (req.method === "POST" && req.url === "/compose/login-code") {
        const chunks: Buffer[] = [];
        for await (const ch of req) chunks.push(ch as Buffer);
        const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
        const locale = (body.locale as string) || "en";
        const email = (body.email as string) || "";
        const code = (body.code as string) || "";
        const appName = (body.app_name as string) || "Taskly";
        const t: (k: string, o?: any) => string = (key: string, o?: any) =>
          translate(key, { lng: locale, ...(o || {}) });
        const element = LoginCodeEmail({ appName, code, email, t });
        const html = render(element, { pretty: true });
        const text = render(element, { plainText: true });
        const subject = t("emails:login_code.subject");
        const out = JSON.stringify({ subject, html, text });
        res.statusCode = 200;
        res.setHeader("Content-Type", "application/json");
        return res.end(out);
      }
      res.statusCode = 404;
      res.end("not found");
    } catch (err: any) {
      res.statusCode = 500;
      res.end(JSON.stringify({ error: err?.message || "internal" }));
    }
  });
  server.listen(port, "0.0.0.0");
}
