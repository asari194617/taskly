import * as React from "react";
import { Layout } from "./Layout.js";

export function DayShiftSkipEmail({
  appName,
  todayDate,
  todayNames,
  nextDate,
  nextNames,
  t,
}: {
  appName: string;
  todayDate: string;
  todayNames: string;
  nextDate: string;
  nextNames: string;
  t: (k: string, o?: any) => string;
}) {
  const title = `${appName} - ${t("emails:dayshift_skip.subject", {
    appName,
  })}`;
  return (
    <Layout title={title}>
      <div className="header">{appName}</div>
      <div className="content">
        <p className="muted">
          {t("emails:dayshift_skip.intro", { date: todayDate })}
        </p>
        <p>
          {t("emails:dayshift_skip.today_line", {
            date: todayDate,
            names: todayNames || "(none)",
          })}
        </p>
        <p>
          {t("emails:dayshift_skip.next_line", {
            date: nextDate,
            names: nextNames || "(none)",
          })}
        </p>
        <p className="muted">{t("emails:dayshift_skip.footer_note")}</p>
      </div>
    </Layout>
  );
}
