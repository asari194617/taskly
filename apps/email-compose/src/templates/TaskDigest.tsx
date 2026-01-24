import * as React from "react";
import { Container, Section, Heading, Text } from "@react-email/components";
import { getT } from "../get-t.js";
import { Layout } from "./Layout.js";

export interface TaskDigestItem {
  whenLabel: string; // 今日/あす/あさって/日付
  subject: string; // 教科名
  title: string; // 課題名
  body?: string; // 詳細説明
  meta?: string; // 教科 + 日付 + ラベルなど（例: "国語 2025/11/04 朝まで"）
  badges?: {
    label: string;
    variant?: "gray" | "red" | "blue";
  }[]; // 先頭のバッジ行（任意）
}

// 重複した"まで まで"を1つにまとめる
const normalizeBody = (s?: string) => {
  if (!s) return "";
  return s.replace(/(まで)(\s*まで)+/g, "$1");
};

export async function TaskDigestEmail({
  items,
  locale,
}: {
  items: TaskDigestItem[];
  locale?: string;
}) {
  const { t } = await getT("tasks-digest", locale);
  let safeItems = items;

  // React EmailのPreviewは必須の配列に対してもundefinedを渡すので、Previewかどうかの判定を行う
  // プレビューなら、ダミーデータを入れる
  if (items === undefined) {
    console.log("Rendering TaskDigestEmail preview with dummy data");
    safeItems = [
      {
        whenLabel: "今日",
        subject: "数理演習2",
        title: "三角関数2のテスト",
        body: "三角関数1の範囲を含む",
        meta: "2025/11/03 1~2限まで",
      },
      {
        whenLabel: "あす",
        subject: "数理演習1",
        title: "図形の再提出",
        body: "",
        meta: "2025/11/04 3~4限まで",
      },
      {
        whenLabel: "あさって",
        subject: "ディジタル工学基礎",
        title: "(再評価対象者のみ)課題提出",
        body: "",
        meta: "2025/11/05 夜まで",
        badges: [{ label: "提出必須", variant: "red" }],
      },
    ];
  }

  const isSingle = safeItems.length === 1;
  const i = safeItems[0];
  const title = isSingle
    ? t("single.title", {
        when: i.whenLabel,
        subject: i.subject,
        task: i.title,
      })
    : t("multi.title", {
        count: safeItems.length,
      });

  safeItems = safeItems.map((it) => ({
    ...it,
    body: normalizeBody(it.body),
  }));

  // group by when_label to resemble the Tasks list sections
  const grouped = new Map<string, TaskDigestItem[]>();
  for (const it of safeItems) {
    const key = it.whenLabel || "";
    if (!grouped.has(key)) grouped.set(key, []);
    grouped.get(key)!.push(it);
  }

  const badgeClass = (variant: "gray" | "red" | "blue" = "gray") => {
    switch (variant) {
      case "red":
        return "bg-red-100 text-red-800 border-red-300";
      case "blue":
        return "bg-blue-100 text-blue-700 border-blue-300";
      default:
        return "bg-gray-100 text-gray-900 border-gray-300";
    }
  };

  return (
    <Layout title={title} locale={locale} footerNote={t("details_note")}>
      <Container className="p-1">
        {[...grouped.entries()].map(([heading, list], gi) => (
          <React.Fragment key={gi}>
            <Section className={gi === 0 ? "pt-1 pb-1.5" : "pt-4 pb-1.5"}>
              <Text className="m-0 text-gray-500 font-semibold">{heading}</Text>
            </Section>
            <Section>
              {list.map((x, idx) => (
                <div
                  key={idx}
                  className="border border-gray-200 rounded-[10px] p-4 mb-3"
                >
                  {/* badges row (optional) */}
                  {x.badges && x.badges.length > 0 ? (
                    <div className="flex gap-2 mb-2">
                      {x.badges.map((b, bi) => (
                        <span
                          key={bi}
                          className={`inline-block px-2 py-[1px] rounded border text-xs font-semibold ${badgeClass(
                            b.variant,
                          )}`}
                        >
                          {b.label}
                        </span>
                      ))}
                    </div>
                  ) : null}

                  {/* title */}
                  <Text className="m-0 text-base font-semibold">{x.title}</Text>

                  {/* meta line: subject + date text */}
                  <Text className="m-0 mt-1 text-gray-500">
                    {x.subject} {x.meta}
                  </Text>

                  {/* description */}
                  {x.body ? (
                    <Text className="m-0 mt-1.5 p-0 text-gray-500">{x.body}</Text>
                  ) : null}
                </div>
              ))}
            </Section>
          </React.Fragment>
        ))}
      </Container>
    </Layout>
  );
}

export default TaskDigestEmail;
