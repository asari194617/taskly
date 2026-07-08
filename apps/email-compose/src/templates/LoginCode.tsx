import * as React from "react";
import { getT } from "../get-t.js";
import { Layout } from "./Layout.js";
import { Section, Text, Hr } from "react-email";

export async function LoginCodeEmail({
  code,
  email,
  locale,
}: {
  code: string;
  email: string;
  locale?: string;
}) {
  const { t } = await getT("login-code", locale);
  const title = t("subject");
  return (
    <Layout title={title} locale={locale}>
      <Section>
        <Text>{t("greeting", { email })}</Text>
        <Text className="text-muted">{t("intro")}</Text>
        <Text>
          <span className="text-lg font-bold">{code}</span>
        </Text>
        <Hr />
        <Text className="text-muted">{t("expiry")}</Text>
        <Text className="text-muted">{t("no_share")}</Text>
      </Section>
    </Layout>
  );
}

export default LoginCodeEmail;
