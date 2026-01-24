import * as React from "react";
import { getT } from "../get-t.js";
import { Layout } from "./Layout.js";
import { Container } from "@react-email/components";

export const DayShiftSkipEmail = async ({
  todayDate,
  todayNames,
  nextDate,
  nextNames,
  locale,
}: {
  todayDate: string;
  todayNames: string;
  nextDate: string;
  nextNames: string;
  locale?: string;
}) => {
  const { t } = await getT("dayshift-skip", locale);
  const title = t("subject");
  return (
    <Layout title={title} locale={locale}>
      <Container className="content">
        <p className="muted">{t("intro", { date: todayDate })}</p>
        <p>
          {t("today_line", {
            date: todayDate,
            names: todayNames || t("none_label"),
          })}
        </p>
        <p>
          {t("next_line", {
            date: nextDate,
            names: nextNames || t("none_label"),
          })}
        </p>
        <p className="muted">{t("details_note")}</p>
      </Container>
    </Layout>
  );
};

export default DayShiftSkipEmail;