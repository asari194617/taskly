import * as React from "react";
import { getT } from "../get-t.js";
import { Layout } from "./Layout.js";
import { Text, Section, Link } from "react-email";

type Highlight = {
  id?: string | number;
  title?: string;
  category?: string;
  severity?: string;
};

export async function ModerationAlertEmail({
  totalFlagged,
  autoHidden,
  reviewUrl,
  highlights,
  locale,
}: {
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
      <Section>
        <Text className="text-muted">{t("intro")}</Text>
        <Text>
          {t("summary", {
            total: totalFlagged,
            hidden: autoHidden,
          })}
        </Text>
        {hs.length > 0 && (
          <Section>
            <Text className="text-muted">{t("highlights")}</Text>
            <ul>
              {hs.map((h, i) => (
                <li key={i}>
                  {h.title ? <strong>{h.title}</strong> : null}
                  {h.category ? <span> — {h.category}</span> : null}
                  {h.severity ? <span> ({h.severity})</span> : null}
                </li>
              ))}
            </ul>
          </Section>
        )}
        {reviewUrl ? (
          <Text>
            <Link className="button" href={reviewUrl}>
              {t("review_button")}
            </Link>
          </Text>
        ) : null}
        <Text className="muted">{t("details_note")}</Text>
      </Section>
    </Layout>
  );
}

export default ModerationAlertEmail;
