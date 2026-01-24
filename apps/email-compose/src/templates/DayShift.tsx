import { Section, Text } from "@react-email/components";
import { getT } from "../get-t.js";
import { Layout } from "./Layout.js";
import { Table } from "../components/table.js";

interface DayShiftProps {
  members: string[];
  events?: string[];
  locale?: string;
}

export const DayShiftEmail = async (props: DayShiftProps) => {
  const { t } = await getT("day-shift", props.locale);
  // React EmailのPreviewは配列に対してもundefinedを渡すので、最初に除外しておく
  const memberStr = props.members !== undefined ? props.members.join(", ") : t("events_none");
  const eventStr = props.events !== undefined && props.events.length > 0 ? props.events.join(", ") : t("events_none");
  
  return (
    <Layout
      title={t("title")}
      preview={t("preview")}
      locale={props.locale}
      footerNote={t("notification_prefs_can_change")}
    >
      <Section>
        <Section>
          <Text className="m-0 p-0">{t("intro")}</Text>
        </Section>
      </Section>
      <Table
        title={t("table_title")}
        rows={[
          { 
            title: t("members_label"), 
            value: memberStr
          },
          {
            title: t("events_label"),
            value: eventStr
          },
        ]}
      />
    </Layout>
  );
};

export default DayShiftEmail;
