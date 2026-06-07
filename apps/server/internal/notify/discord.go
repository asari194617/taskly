package notify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SendReportToDiscord posts a simple content payload to the provided webhook URL.
func SendReportToDiscord(hook string, taskID uint, reason string) error {
	if strings.TrimSpace(hook) == "" {
		return nil
	}
	payload := map[string]any{"content": "[Report] TaskID: " + fmtTaskID(taskID) + " Reason: " + reason}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", hook, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
	return nil
}

func fmtTaskID(id uint) string {
	return fmtUint(id)
}

func fmtUint(v uint) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.Trim(fmt.Sprintf("%d", v), "\n"), "\r", ""))
}
