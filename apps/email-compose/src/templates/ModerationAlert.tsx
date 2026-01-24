import * as React from "react";
import { getT } from "../get-t.js";
import { Layout } from "./Layout.js";

type Highlight = {
  id?: string | number;
  title?: string;
  category?: string;
  severity?: string;
};

export async function ModerationAlertEmail({
  appName,
  totalFlagged,
  autoHidden,
  reviewUrl,
  highlights,
  locale,
}: {
  appName: string;
  totalFlagged: number;
  autoHidden: number;
  reviewUrl?: string;
  highlights?: Highlight[];
  locale?: string;
}) {
  const { t } = await getT("moderation-alert", locale);
  const title = t("title");
  const hs = Array.isArray(highlights) ? highlights.slice(0, 5) : [];
  return (
    <Layout title={title} locale={locale}>
      <div className="header">{appName}</div>
      <div className="content">
        <p className="muted">{t("intro")}</p>
        <p>
          {t("summary", {
            total: totalFlagged,
            hidden: autoHidden,
          })}
        </p>
        {hs.length > 0 && (
          <div>
            <p className="muted">{t("highlights")}</p>
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
              {t("review_button")}
            </a>
          </p>
        ) : null}
        <p className="muted">{t("details_note")}</p>
      </div>
    </Layout>
  );
}

export default ModerationAlertEmail;