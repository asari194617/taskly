import { Text, Section, Row, Column } from "react-email";

interface CommonFrameProps {
  title: string;
  rows: {
    title: string;
    value: string;
  }[];
}

export const Table = (props: CommonFrameProps) => {
  return (
    <Section>
      <Text className="text-gray-600 text-sm">{props.title}</Text>
      <Section className="mt-2 mb-4 border border-gray-300 rounded">
        {props.rows.map((row, index) => (
          <Row key={index}>
            <Column
              align="left"
              className="mb-[8px] p-[8px] text-gray-700 font-bold"
            >
              {row.title}
            </Column>
            <Column
              align="right"
              className="mb-[8px] p-[8px] text-gray-700 font-normal"
              style={{
                // Allow long, unbroken strings (e.g., URLs, IDs) to wrap in email clients
                whiteSpace: "normal",
                wordBreak: "break-word", // broad support (Gmail/Apple Mail)
                overflowWrap: "break-word", // legacy alias support
                wordWrap: "break-word", // Outlook legacy support
              }}
            >
              {row.value}
            </Column>
          </Row>
        ))}
      </Section>
    </Section>
  );
};

export default Table;
