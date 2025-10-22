import * as React from "react";
import { Layout } from "./Layout.js";

type Highlight = {
  id?: string | number;
  title?: string;
  category?: string;
  severity?: string;
};

export function ModerationAlertEmail({
  appName,
  totalFlagged,
  autoHidden,
  reviewUrl,
  highlights,
  t,
}: {
  appName: string;
  totalFlagged: number;
  autoHidden: number;
  reviewUrl?: string;
  highlights?: Highlight[];
  t: (k: string, o?: any) => string;
}) {
  const title = `${appName} - ${t("emails:moderation_alert.title")}`;
  const hs = Array.isArray(highlights) ? highlights.slice(0, 5) : [];
  return (
    <Layout title={title}>
      <div className="header">{appName}</div>
      <div className="content">
        <p className="muted">{t("emails:moderation_alert.intro")}</p>
        <p>
          {t("emails:moderation_alert.summary", {
            total: totalFlagged,
            hidden: autoHidden,
          })}
        </p>
        {hs.length > 0 && (
          <div>
            <p className="muted">{t("emails:moderation_alert.highlights")}</p>
            <ul>
              {hs.map((h, i) => (
                <li key={i}>
                  {h.title ? <strong>{h.title}</strong> : null}
                  {h.category ? <span> — {h.category}</span> : null}
                  {h.severity ? <span> ({h.severity})</span> : null}
                </li>
              ))}
            </ul>
          </div>
        )}
        {reviewUrl ? (
          <p>
            <a className="button" href={reviewUrl}>
              {t("emails:moderation_alert.review_button")}
            </a>
          </p>
        ) : null}
        <p className="muted">{t("emails:moderation_alert.footer_note")}</p>
      </div>
    </Layout>
  );
}
