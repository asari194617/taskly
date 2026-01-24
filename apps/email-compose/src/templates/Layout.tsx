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

// Simple base HTML with neutral styling similar to shadcn/ui typography
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
  // デフォルトのままだとmm/dd/yyyy形式になるので、yyyy/mm/dd形式で構成
  const date = datetime.toLocaleString("ja-JP", {
    timeZone: "Asia/Tokyo",
    year: "numeric"
  }).concat("/").concat(
    datetime.toLocaleString("ja-JP", {
      timeZone: "Asia/Tokyo",
      month: "2-digit"
    })
  ).concat("/").concat(
    datetime.toLocaleString("ja-JP", {
      timeZone: "Asia/Tokyo",
      day: "2-digit"
    })
  );

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
          <Section className="w-full max-w-sm">
            <Section className="rounded-xl bg-white p-4">
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
              <p className="text-xs text-center text-gray-500 m-0">
                {resolvedFooter} 
                このメールは自動送信されています。{date}時点での情報です。追加、更新、または削除される場合があります。
              </p>
            </Section>
          </Section>
        </Tailwind>
      </Section>
    </Html>
  );
}

export default Layout;
