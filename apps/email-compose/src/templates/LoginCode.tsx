import * as React from "react";
import { Layout } from "./Layout.js";

export function LoginCodeEmail({
  appName,
  code,
  email,
  t,
}: {
  appName: string;
  code: string;
  email: string;
  t: (k: string, o?: any) => string;
}) {
  const title = `${appName} - ${t("emails:login_code.subject")}`;
  return (
    <Layout title={title}>
      <div className="header">{appName}</div>
      <div className="content">
        <p>{t("emails:login_code.greeting", { email })}</p>
        <p className="muted">{t("emails:login_code.intro")}</p>
        <p>
          <span className="code">{code}</span>
        </p>
        <p className="muted">{t("emails:login_code.expiry")}</p>
      </div>
    </Layout>
  );
}
