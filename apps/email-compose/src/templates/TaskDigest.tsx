import * as React from "react";
import { Container, Section, Heading, Text, Hr } from "@react-email/components";
import { Layout } from "./Layout.js";

export type TaskDigestItem = {
  when_label: string; // 今日/あす/あさって/日付
  subject: string; // 教科名
  title: string; // 課題名
  body?: string; // 詳細説明
  meta?: string; // 教科 + 日付 + ラベルなど（例: "国語 2025/11/04 朝まで"）
  badges?: Array<{ label: string; variant?: "gray" | "red" | "blue" }>; // 先頭のバッジ行（任意）
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
  // normalize duplicated suffix like "まで まで" -> "まで"
  const normalizeBody = (s?: string) => {
    if (!s) return "";
    return s.replace(/(まで)(\s*まで)+/g, "$1");
  };
  const safeItems = items.map((it) => ({
    ...it,
    body: normalizeBody(it.body),
  }));
  const isSingle = safeItems.length === 1;
  const i = safeItems[0];
  const title = isSingle
    ? `${appName} - ${t("emails:tasks_digest.single.title", {
        when: i.when_label,
        subject: i.subject,
        task: i.title,
      })}`
    : `${appName} - ${t("emails:tasks_digest.multi.title", {
        count: safeItems.length,
      })}`;

  // group by when_label to resemble the Tasks list sections
  const grouped = new Map<string, TaskDigestItem[]>();
  for (const it of safeItems) {
    const key = it.when_label || "";
    if (!grouped.has(key)) grouped.set(key, []);
    grouped.get(key)!.push(it);
  }

  const badgeStyle = (variant: "gray" | "red" | "blue" = "gray") => {
    switch (variant) {
      case "red":
        return {
          backgroundColor: "#FEE2E2",
          color: "#991B1B",
        } as React.CSSProperties;
      case "blue":
        return {
          backgroundColor: "#DBEAFE",
          color: "#1D4ED8",
        } as React.CSSProperties;
      default:
        return {
          backgroundColor: "#F3F4F6",
          color: "#111827",
        } as React.CSSProperties;
    }
  };

  return (
    <Layout title={title}>
      <Container>
        <Section style={{ paddingTop: 8, paddingBottom: 8 }}>
          <Heading as="h1" style={{ margin: 0, fontSize: 18 }}>
            {isSingle
              ? t("emails:tasks_digest.single.title", {
                  when: i.when_label,
                  subject: i.subject,
                  task: i.title,
                })
              : t("emails:tasks_digest.multi.title", {
                  count: safeItems.length,
                })}
          </Heading>
        </Section>

        {[...grouped.entries()].map(([heading, list], gi) => (
          <React.Fragment key={gi}>
            <Section
              style={{ paddingTop: gi === 0 ? 4 : 16, paddingBottom: 6 }}
            >
              <Text style={{ margin: 0, color: "#6B7280", fontWeight: 600 }}>
                {heading}
              </Text>
            </Section>
            <Section>
              {list.map((x, idx) => (
                <div
                  key={idx}
                  style={{
                    border: "1px solid #E5E7EB",
                    borderRadius: 10,
                    padding: 16,
                    marginBottom: 12,
                  }}
                >
                  {/* badges row (optional) */}
                  {x.badges && x.badges.length > 0 ? (
                    <div style={{ display: "flex", gap: 8, marginBottom: 8 }}>
                      {x.badges.map((b, bi) => (
                        <span
                          key={bi}
                          style={{
                            display: "inline-block",
                            padding: "4px 8px",
                            borderRadius: 9999,
                            fontSize: 12,
                            fontWeight: 600,
                            ...badgeStyle(b.variant),
                          }}
                        >
                          {b.label}
                        </span>
                      ))}
                    </div>
                  ) : null}

                  {/* title */}
                  <Text style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>
                    {x.title}
                  </Text>

                  {/* meta line: subject + date text */}
                  <Text style={{ marginTop: 4, color: "#6B7280" }}>
                    {x.meta ?? x.subject}
                  </Text>

                  {/* description */}
                  {x.body ? (
                    <Text style={{ marginTop: 6, color: "#6B7280" }}>
                      {x.body}
                    </Text>
                  ) : null}
                </div>
              ))}
            </Section>
          </React.Fragment>
        ))}

        <Section style={{ paddingTop: 8 }}>
          <Text style={{ color: "#6B7280" }}>
            {t("emails:tasks_digest.footer_note")}
          </Text>
        </Section>
      </Container>
    </Layout>
  );
}
