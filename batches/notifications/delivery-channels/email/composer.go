package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	protoparse "github.com/jhump/protoreflect/desc/protoparse"
	dynmsg "github.com/jhump/protoreflect/dynamic"
	grpcdyn "github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func buildComposePayload(locale, appName string, items []QueueItem) composePayload {
	out := composePayload{Locale: locale, AppName: appName}
	for _, it := range items {
		m := map[string]any{
			"when_label": it.WhenLabel,
			"subject":    it.Subject,
			"title":      it.Title,
			"body":       it.Body,
		}
		if it.Meta.Valid {
			m["meta"] = it.Meta.String
		}
		if it.Badges.Valid && strings.TrimSpace(it.Badges.String) != "" {
			var arr []map[string]any
			if err := json.Unmarshal([]byte(it.Badges.String), &arr); err == nil {
				m["badges"] = arr
			}
		}
		out.Items = append(out.Items, m)
	}
	return out
}

// composeTaskDigestGRPC calls email-compose gRPC (descriptor-based) to render digest
func composeTaskDigestGRPC(locale, appName string, items []QueueItem) (html string, text string, err error) {
	addr := strings.TrimSpace(os.Getenv("EMAIL_COMPOSE_GRPC_ADDR"))
	if addr == "" {
		addr = "email-compose:50051"
	}
	cc, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", "", err
	}
	defer cc.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(parseInt(valueOr(os.Getenv("SMTP_TIMEOUT"), "60")))*time.Second)
	defer cancel()

	protoDir := "/workspace/apps/email-compose/src/proto"
	if _, errStat := os.Stat(protoDir); errStat != nil {
		protoDir = "../../../apps/email-compose/src/proto"
	}
	parser := protoparse.Parser{ImportPaths: []string{protoDir}}
	fds, pErr := parser.ParseFiles("email.proto")
	if pErr != nil || len(fds) == 0 {
		return "", "", fmt.Errorf("proto parse error: %v", pErr)
	}
	fd := fds[0]
	svc := fd.FindService("emailcompose.EmailComposer")
	if svc == nil {
		return "", "", fmt.Errorf("service emailcompose.EmailComposer not found in proto")
	}
	m := svc.FindMethodByName("ComposeTaskDigest")
	if m == nil {
		return "", "", fmt.Errorf("ComposeTaskDigest not found")
	}
	payload := buildComposePayload(locale, appName, items)
	factory := dynmsg.NewMessageFactoryWithDefaults()
	reqMsgDyn := factory.NewMessage(m.GetInputType()).(*dynmsg.Message)
	reqMsgDyn.SetFieldByName("locale", payload.Locale)
	reqMsgDyn.SetFieldByName("app_name", payload.AppName)
	itmMsgDesc := m.GetInputType().FindFieldByName("items").GetMessageType()
	for _, it := range payload.Items {
		itm := factory.NewMessage(itmMsgDesc).(*dynmsg.Message)
		itm.SetFieldByName("when_label", it["when_label"])
		itm.SetFieldByName("subject", it["subject"])
		itm.SetFieldByName("title", it["title"])
		if v, ok := it["body"]; ok {
			itm.SetFieldByName("body", v)
		}
		if v, ok := it["meta"]; ok {
			itm.SetFieldByName("meta", v)
		}
		if v, ok := it["badges"]; ok {
			if arr, ok := v.([]map[string]any); ok {
				bdMsgDesc := itmMsgDesc.FindFieldByName("badges").GetMessageType()
				for _, b := range arr {
					bm := factory.NewMessage(bdMsgDesc).(*dynmsg.Message)
					bm.SetFieldByName("label", b["label"])
					if vv, ok := b["variant"]; ok {
						bm.SetFieldByName("variant", vv)
					}
					itm.AddRepeatedFieldByName("badges", bm)
				}
			}
		}
		reqMsgDyn.AddRepeatedFieldByName("items", itm)
	}
	stub := grpcdyn.NewStub(cc)
	resp, err := stub.InvokeRpc(ctx, m, reqMsgDyn)
	if err != nil {
		return "", "", err
	}
	if dm, ok := resp.(*dynmsg.Message); ok {
		h := dm.GetFieldByName("html")
		t := dm.GetFieldByName("text")
		hs, _ := h.(string)
		ts, _ := t.(string)
		return hs, ts, nil
	}
	return "", "", fmt.Errorf("unexpected response type")
}
