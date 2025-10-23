import * as React from "react";
import { Layout } from "./Layout.js";

export type TaskDigestItem = {
  when_label: string; // 今日/あす/あさって/日付
  subject: string; // 教科名
  title: string; // 課題名
  body?: string; // 〜まで 等
};

export function TaskDigestEmail({
  appName = "Taskly",
  items,
  t,
}: {
  appName?: string;
  items: TaskDigestItem[];
  t: (k: string, o?: any) => string;
}) {
  const isSingle = items.length === 1;
  const i = items[0];
  const title = isSingle
    ? `${appName} - ${t("emails:tasks_digest.single.title", {
        when: i.when_label,
        subject: i.subject,
        task: i.title,
      })}`
    : `${appName} - ${t("emails:tasks_digest.multi.title", {
        count: items.length,
      })}`;
  return (
    <Layout title={title}>
      <div className="header">{appName}</div>
      <div className="content">
        {isSingle ? (
          <>
            <h1 style={{ margin: 0, fontSize: 18 }}>
              {t("emails:tasks_digest.single.title", {
                when: i.when_label,
                subject: i.subject,
                task: i.title,
              })}
            </h1>
            <p className="muted" style={{ marginTop: 6 }}>
              {i.body || ""}
            </p>
          </>
        ) : (
          <>
            <h1 style={{ margin: 0, fontSize: 18 }}>
              {t("emails:tasks_digest.multi.title", { count: items.length })}
            </h1>
            <ul style={{ marginTop: 8, paddingLeft: 18 }}>
              {items.map((x, idx) => (
                <li key={idx}>
                  <span style={{ fontWeight: 600 }}>[{x.when_label}]</span>{" "}
                  <span style={{ fontWeight: 600 }}>{x.subject}</span>:{" "}
                  {x.title}
                  {x.body ? <span> - {x.body}</span> : null}
                </li>
              ))}
            </ul>
          </>
        )}
        <p className="muted" style={{ marginTop: 12 }}>
          {t("emails:tasks_digest.footer_note")}
        </p>
      </div>
    </Layout>
  );
}
