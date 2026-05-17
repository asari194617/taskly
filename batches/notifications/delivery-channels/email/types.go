package main

type composePayload struct {
	Locale  string           `json:"locale"`
	AppName string           `json:"app_name"`
	Items   []map[string]any `json:"items"`
}

// EmailMessage represents the parts needed to build a multipart MIME message
type EmailMessage struct {
	From     string
	To       string
	Subject  string
	Text     string
	HTML     string
	Boundary string // optional
}

type DeliverOptions struct {
	Locale  string
	AppName string
}
