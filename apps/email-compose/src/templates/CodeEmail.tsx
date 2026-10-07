import * as React from "react";
import { Section, Text, Hr } from "react-email";
import { getT } from "../get-t.js";
import { Layout } from "./Layout.js";

export async function CodeEmail({
  code,
  email,
  locale,
  expiryMinutes,
  namespace,
}: {
  code: string;
  email: string;
  locale?: string;
  expiryMinutes: number;
  namespace: "login-code" | "security-code" | "email-verification-code";
}) {
  const { t } = await getT(namespace, locale);
  return (
    <Layout title={t("subject")} locale={locale}>
      <Section>
        <Text>{t("greeting", { email })}</Text>
        <Text className="text-muted">{t("intro")}</Text>
        <Text>
          {t("code_label")} {" "}
          <span className="text-lg font-bold">{code}</span>
        </Text>
        <Hr />
        <Text className="text-muted">{t("expiry", { minutes: expiryMinutes })}</Text>
        <Text className="text-muted">{t("no_share")}</Text>
      </Section>
    </Layout>
  );
}
