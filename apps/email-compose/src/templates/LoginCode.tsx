import * as React from "react";
import { getT } from "../get-t.js";
import { Layout } from "./Layout.js";

export async function LoginCodeEmail({
  appName,
  code,
  email,
  locale,
}: {
  appName: string;
  code: string;
  email: string;
  locale?: string;
}) {
  const { t } = await getT("login-code", locale);
  const title = t("subject");
  return (
    <Layout title={title} locale={locale}>
      <div className="header">{appName}</div>
      <div className="content">
        <p>{t("greeting", { email })}</p>
        <p className="muted">{t("intro")}</p>
        <p>
          <span className="code">{code}</span>
        </p>
        <p className="muted">{t("expiry")}</p>
      </div>
    </Layout>
  );
}

export default LoginCodeEmail;