import * as React from "react";
import { Html } from "@react-email/components";

// Simple base HTML with neutral styling similar to shadcn/ui typography
export function Layout({
  children,
  title,
}: {
  children: React.ReactNode;
  title: string;
}) {
  return (
    <Html lang="en">
      <head>
        <meta charSet="utf-8" />
        <meta httpEquiv="x-ua-compatible" content="ie=edge" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>{title}</title>
        <style>
          {`
            :root { color-scheme: light only; }
            body { margin: 0; padding: 0; font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Ubuntu, Cantarell, Noto Sans, Helvetica Neue, Arial, "Apple Color Emoji", "Segoe UI Emoji"; background: #f8fafc; color: #0f172a; }
            .container { max-width: 640px; margin: 0 auto; padding: 24px; }
            .card { background: white; border-radius: 12px; border: 1px solid #e2e8f0; overflow: hidden; }
            .header { padding: 20px 24px; border-bottom: 1px solid #e2e8f0; font-weight: 600; font-size: 18px; }
            .content { padding: 24px; font-size: 14px; line-height: 1.6; }
            .muted { color: #64748b; }
            .button { display: inline-block; background: #111827; color: white; padding: 10px 16px; border-radius: 8px; text-decoration: none; font-weight: 600; }
            .code { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace; background: #f1f5f9; border: 1px solid #e2e8f0; padding: 6px 8px; border-radius: 6px; }
            .footer { text-align: center; padding: 16px; color: #94a3b8; font-size: 12px; }
          `}
        </style>
      </head>
      <body>
        <div className="container">
          <div className="card">{children}</div>
          <div className="footer">
            <img src="https://cdn.taskly.iy0306.com/assets/logo.jpg" alt="ロゴ" />
            これは自動送信されたメールです。監視されていないメールボックスのため、返信しないでください。
          </div>
        </div>
      </body>
    </Html>
  );
}
