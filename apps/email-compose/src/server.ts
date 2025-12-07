import path from "node:path";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";
import { render } from "@react-email/render";
import { LoginCodeEmail } from "./templates/LoginCode.js";
import { ModerationAlertEmail } from "./templates/ModerationAlert.js";
import { DayShiftSkipEmail } from "./templates/DayShiftSkip.js";
import {
  TaskDigestEmail,
  type TaskDigestItem,
} from "./templates/TaskDigest.js";
import { t as translate } from "./i18n.js";

const PROTO_PATH = path.join(process.cwd(), "dist/proto/email.proto");

type GrpcServerOptions = { host: string; port: number };

export async function startGrpcServer({ host, port }: GrpcServerOptions) {
  const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
    keepCase: true,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true,
  });
  const proto = (grpc.loadPackageDefinition(packageDefinition) as any)
    .emailcompose;

  const server = new grpc.Server();

  const serviceImpl = {
    ComposeLoginCode: (
      call: grpc.ServerUnaryCall<any, any>,
      callback: grpc.sendUnaryData<any>
    ) => {
      try {
        const locale = (call.request?.locale as string) || "en";
        const email = call.request?.email as string;
        const code = call.request?.code as string;
        const appName = (call.request?.app_name as string) || "Taskly";

        const t: (k: string, o?: any) => string = (key: string, o?: any) =>
          translate(key, { lng: locale, ...(o || {}) });

        const element = LoginCodeEmail({ appName, code, email, t });
        const html = render(element, { pretty: true });
        const text = render(element, { plainText: true });
        const subject = t("emails:login_code.subject");

        callback(null, { subject, html, text });
      } catch (err) {
        callback(
          {
            code: grpc.status.INTERNAL,
            message: (err as Error).message,
          } as any,
          null
        );
      }
    },
    ComposeTaskDigest: (
      call: grpc.ServerUnaryCall<any, any>,
      callback: grpc.sendUnaryData<any>
    ) => {
      try {
        const locale = (call.request?.locale as string) || "ja";
        const appName = (call.request?.app_name as string) || "Taskly";
        const items: TaskDigestItem[] = Array.isArray(call.request?.items)
          ? (call.request.items as TaskDigestItem[])
          : [];

        const t: (k: string, o?: any) => string = (key: string, o?: any) =>
          translate(key, { lng: locale, ...(o || {}) });
        const element = TaskDigestEmail({ appName, items, t });
        const html = render(element, { pretty: true });
        const text = render(element, { plainText: true });

        let subject: string;
        if (items.length === 1) {
          const i = items[0];
          subject = `【${i.when_label}まで】${i.subject}: ${i.title}`;
        } else {
          subject = `期限が近い課題が${items.length}件あります`;
        }

        callback(null, { subject, html, text });
      } catch (err) {
        callback(
          {
            code: grpc.status.INTERNAL,
            message: (err as Error).message,
          } as any,
          null
        );
      }
    },
    ComposeModerationAlert: (
      call: grpc.ServerUnaryCall<any, any>,
      callback: grpc.sendUnaryData<any>
    ) => {
      try {
        const locale = (call.request?.locale as string) || "ja";
        const appName = (call.request?.app_name as string) || "Taskly";
        const totalFlagged = Number(call.request?.total_flagged || 0);
        const autoHidden = Number(call.request?.auto_hidden || 0);
        const reviewUrl = (call.request?.review_url as string) || "";
        const highlights: Array<{
          title?: string;
          category?: string;
          severity?: string;
          id?: string | number;
        }> = Array.isArray(call.request?.highlights)
          ? (call.request.highlights as any[])
          : [];

        const t: (k: string, o?: any) => string = (key: string, o?: any) =>
          translate(key, { lng: locale, ...(o || {}) });
        const element = ModerationAlertEmail({
          appName,
          totalFlagged,
          autoHidden,
          reviewUrl,
          highlights,
          t,
        });
        const html = render(element, { pretty: true });
        const text = render(element, { plainText: true });
        const subject = t("emails:moderation_alert.subject", { appName });

        callback(null, { subject, html, text });
      } catch (err) {
        callback(
          {
            code: grpc.status.INTERNAL,
            message: (err as Error).message,
          } as any,
          null
        );
      }
    },
    ComposeDayShiftSkip: (
      call: grpc.ServerUnaryCall<any, any>,
      callback: grpc.sendUnaryData<any>
    ) => {
      try {
        const locale = (call.request?.locale as string) || "ja";
        const appName = (call.request?.app_name as string) || "Taskly";
        const todayDate = (call.request?.today_date as string) || "";
        const todayNames = (call.request?.today_names as string) || "";
        const nextDate = (call.request?.next_date as string) || "";
        const nextNames = (call.request?.next_names as string) || "";

        const t: (k: string, o?: any) => string = (key: string, o?: any) =>
          translate(key, { lng: locale, ...(o || {}) });
        const element = DayShiftSkipEmail({
          appName,
          todayDate,
          todayNames,
          nextDate,
          nextNames,
          t,
        });
        const html = render(element, { pretty: true });
        const text = render(element, { plainText: true });
        const subject = t("emails:dayshift_skip.subject", { appName });

        callback(null, { subject, html, text });
      } catch (err) {
        callback(
          {
            code: grpc.status.INTERNAL,
            message: (err as Error).message,
          } as any,
          null
        );
      }
    },
  };

  server.addService(proto.EmailComposer.service, serviceImpl);
  const addr = `${host}:${port}`;
  await new Promise<void>((resolve, reject) => {
    server.bindAsync(
      addr,
      grpc.ServerCredentials.createInsecure(),
      (err: any, _port: any) => {
        if (err) return reject(err);
        resolve();
      }
    );
  });
  server.start();
}
