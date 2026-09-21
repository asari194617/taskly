package handler

import "testing"

func TestParseDMARCXML(t *testing.T) {
	report := []byte(`<?xml version="1.0"?>
<feedback>
  <report_metadata><date_range><begin>0</begin><end>0</end></date_range></report_metadata>
  <policy_published><domain>example.test</domain></policy_published>
  <record><row><count>3</count></row><policy_evaluated><disposition>none</disposition><dkim>pass</dkim><spf>pass</spf></policy_evaluated></record>
  <record><row><count>2</count></row><policy_evaluated><disposition>reject</disposition><dkim>pass</dkim><spf>fail</spf></policy_evaluated></record>
</feedback>`)

	got := parseDMARCXML(report)
	if got == nil {
		t.Fatal("parseDMARCXML() returned nil")
	}
	if got.ReportDomain != "example.test" || got.Total != 5 || got.Pass != 3 || got.Failed != 2 {
		t.Fatalf("parseDMARCXML() = %#v", got)
	}
}

func TestExtractBouncedRecipientsDeduplicates(t *testing.T) {
	got := extractBouncedRecipients("Final-Recipient: rfc822; User@example.test\nOriginal-Recipient: rfc822; user@example.test")
	if len(got) != 1 || got[0] != "user@example.test" {
		t.Fatalf("extractBouncedRecipients() = %v", got)
	}
}
