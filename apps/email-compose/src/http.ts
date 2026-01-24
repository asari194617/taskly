import http from "node:http";
import { render } from "@react-email/render";
import { LoginCodeEmail } from "./templates/LoginCode.js";
import { ModerationAlertEmail } from "./templates/ModerationAlert.js";
import {
  TaskDigestEmail,
  type TaskDigestItem,
} from "./templates/TaskDigest.js";
import { getT } from "./get-t.js";

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
        const { t } = await getT("login-code", locale);
        const element = await LoginCodeEmail({ appName, code, email, locale });
        const html = render(element, { pretty: true });
        const text = render(element, { plainText: true });
        const subject = t("subject");
        const out = JSON.stringify({ subject, html, text });
        res.statusCode = 200;
        res.setHeader("Content-Type", "application/json");
        return res.end(out);
      }
      if (req.method === "POST" && req.url === "/compose/moderation-alert") {
        const chunks: Buffer[] = [];
        for await (const ch of req) chunks.push(ch as Buffer);
        const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
        const locale = (body.locale as string) || "en";
        const appName = (body.app_name as string) || "Taskly";
        const totalFlagged = Number(body.total_flagged || 0);
        const autoHidden = Number(body.auto_hidden || 0);
        const reviewUrl = (body.review_url as string) || "";
        const highlights: Array<{
          title?: string;
          category?: string;
          severity?: string;
          id?: string | number;
        }> = Array.isArray(body.highlights) ? body.highlights : [];
        const { t } = await getT("moderation-alert", locale);
        const element = await ModerationAlertEmail({
          appName,
          totalFlagged,
          autoHidden,
          reviewUrl,
          highlights,
          locale,
        });
        const html = render(element, { pretty: true });
        const text = render(element, { plainText: true });
        const subject = t("subject", { appName });
        const out = JSON.stringify({ subject, html, text });
        res.statusCode = 200;
        res.setHeader("Content-Type", "application/json");
        return res.end(out);
      }
      if (req.method === "POST" && req.url === "/compose/task-digest") {
        const chunks: Buffer[] = [];
        for await (const ch of req) chunks.push(ch as Buffer);
        const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
        const locale = (body.locale as string) || "ja";
        const appName = (body.app_name as string) || "Taskly";
        const items: TaskDigestItem[] = Array.isArray(body.items)
          ? body.items
          : [];
        const { t } = await getT("tasks-digest", locale);
        const element = await TaskDigestEmail({ items, locale });
        const html = render(element, { pretty: true });
        const text = render(element, { plainText: true });
        let subject: string;
        if (items.length === 1) {
          const i = items[0];
          subject = t("single.title", {
            when: i.whenLabel,
            subject: i.subject,
            task: i.title,
          });
        } else {
          subject = t("multi.title", { count: items.length });
        }
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
