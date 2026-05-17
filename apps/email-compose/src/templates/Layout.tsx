import * as React from "react";
import {
  Html,
  Font,
  Head,
  Section,
  Tailwind,
  pixelBasedPreset,
  Heading,
  Preview,
  Img,
} from "@react-email/components";

export function Layout({
  children,
  title,
  preview,
  locale,
  footerNote,
}: {
  children: React.ReactNode;
  title: string;
  preview?: string;
  locale?: string;
  footerNote?: string;
}) {
  const datetime = new Date();
  // YYYY/mm/dd HH:MM形式で構成
  const date = datetime.toLocaleString("ja-JP", {
    timeZone: "Asia/Tokyo",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour12: false,
  });

  const time = datetime.toLocaleString("ja-JP", {
    timeZone: "Asia/Tokyo",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });

  const resolvedFooter = footerNote?.replace("{{date}}", date) || "";
  return (
    <Html
      lang={locale || "en"}
      style={{
        backgroundColor: "#f7f7f7",
      }}
    >
      {preview && <Preview>{preview}</Preview>}
      <Head>
        <title>{title}</title>
        <Font
          fontFamily="Roboto"
          fallbackFontFamily="Verdana"
          webFont={{
            url: "https://fonts.gstatic.com/s/roboto/v27/KFOmCnqEu92Fr1Mu4mxKKTU1Kg.woff2",
            format: "woff2",
          }}
          fontWeight={400}
          fontStyle="normal"
        />
      </Head>
      <Section>
        <Tailwind
          config={{
            presets: [pixelBasedPreset],
            theme: {
              extend: {
                colors: {
                  brand: "#1155cc",
                },
              },
            },
          }}
        >
          {/* コンテンツ */}
          <Section className="w-full max-w-md">
            <Section className="rounded-xl bg-white p-4 border border-neutral-200">
              <Img
                src="https://cdn.taskly.iy0306.com/assets/logo.jpg"
                alt="Taskly Logo"
                className="mb-2 h-5"
              />
              {/* ヘッダー */}
              <Heading
                as="h1"
                className="text-primary font-bold text-lg m-0 pb-2"
              >
                {title}
              </Heading>
              {children}
            </Section>
            {/* フッター */}
            <Section className="pt-6">
              <p className="text-xs text-center text-gray-300 m-0">
                {resolvedFooter}
                <br />
                このメールは自動送信されています。このメールアドレスに返信いただいても、メールを確認できません。
                <br />
                {time}
                時点での情報です。追加、更新、または削除される場合があります。
              </p>
            </Section>
          </Section>
        </Tailwind>
      </Section>
    </Html>
  );
}

export default Layout;
