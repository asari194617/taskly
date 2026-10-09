import * as path from "node:path";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";
import { render } from "@react-email/render";
import { LoginCodeEmail } from "./templates/LoginCode.js";
import { SecurityCodeEmail } from "./templates/SecurityCode.js";
import { EmailVerificationCodeEmail } from "./templates/EmailVerificationCode.js";
import { ModerationAlertEmail } from "./templates/ModerationAlert.js";
import { DayShiftSkipEmail } from "./templates/DayShiftSkip.js";
import { DayShiftEmail } from "./templates/DayShift.js";
import {
  TaskDigestEmail,
  type TaskDigestItem,
} from "./templates/TaskDigest.js";
import { getT } from "./get-t.js";

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

  const composeCode = (
    namespace: "login-code" | "security-code" | "email-verification-code",
    template: typeof LoginCodeEmail,
    defaultExpiryMinutes: number,
  ) => async (
    call: grpc.ServerUnaryCall<any, any>,
    callback: grpc.sendUnaryData<any>,
  ) => {
    try {
      const locale = (call.request?.locale as string) || "ja";
      const email = call.request?.email as string;
      const code = call.request?.code as string;
      const requestedExpiry = Number(call.request?.expiry_minutes);
      const expiryMinutes = Number.isInteger(requestedExpiry) && requestedExpiry > 0
        ? requestedExpiry : defaultExpiryMinutes;
      const { t } = await getT(namespace, locale);
      const element = await template({ code, email, locale, expiryMinutes });
      const html = await render(element, { pretty: true });
      const text = await render(element, { plainText: true });
      callback(null, { subject: t("subject"), html, text });
    } catch (err) {
      callback({ code: grpc.status.INTERNAL, message: (err as Error).message } as any, null);
    }
  };

  const serviceImpl = {
    ComposeDayShift: async (
      call: grpc.ServerUnaryCall<any, any>,
      callback: grpc.sendUnaryData<any>,
    ) => {
      try {
        const locale = (call.request?.locale as string) || "ja";
        const members: string[] = Array.isArray(call.request?.members)
          ? (call.request.members as string[])
          : [];
        const events: string[] = Array.isArray(call.request?.events)
          ? (call.request.events as string[])
          : [];

        const { t } = await getT("day-shift", locale);
        const element = await DayShiftEmail({ members, events, locale });
        const html = await render(element, { pretty: true });
        const text = await render(element, { plainText: true });
        const subject = t("title");

        callback(null, { subject, html, text });
      } catch (err) {
        callback(
          {
            code: grpc.status.INTERNAL,
            message: (err as Error).message,
          } as any,
          null,
        );
      }
    },
    ComposeLoginCode: composeCode("login-code", LoginCodeEmail, 5),
    ComposeSecurityCode: composeCode("security-code", SecurityCodeEmail, 10),
    ComposeEmailVerificationCode: composeCode("email-verification-code", EmailVerificationCodeEmail, 10),
    ComposeTaskDigest: async (
      call: grpc.ServerUnaryCall<any, any>,
      callback: grpc.sendUnaryData<any>,
    ) => {
      try {
        const locale = (call.request?.locale as string) || "ja";
        const rawItems: any[] = Array.isArray(call.request?.items)
          ? (call.request.items as any[])
          : [];
        const items: TaskDigestItem[] = rawItems.map((it) => ({
          whenLabel: it.when_label ?? it.whenLabel ?? "",
          subject: it.subject,
          title: it.title,
          body: it.body,
          meta: it.meta,
          badges: it.badges,
          completed: it.completed ?? false,
        }));

        const { t } = await getT("tasks-digest", locale);
        const element = await TaskDigestEmail({ items, locale });
        const html = await render(element, { pretty: true });
        const text = await render(element, { plainText: true });

        let subject: string;
        if (items.length === 1) {
          const i = items[0];
          subject = t("single.title", {
            when: i.whenLabel,
            subject: i.subject,
            task: i.title,
          });
        } else {
          subject = t("multi.title", { count: items.length });
        }

        callback(null, { subject, html, text });
      } catch (err) {
        callback(
          {
            code: grpc.status.INTERNAL,
            message: (err as Error).message,
          } as any,
          null,
        );
      }
    },
    ComposeModerationAlert: async (
      call: grpc.ServerUnaryCall<any, any>,
      callback: grpc.sendUnaryData<any>,
    ) => {
      try {
        const locale = (call.request?.locale as string) || "ja";
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

        const { t } = await getT("moderation-alert", locale);
        const element = await ModerationAlertEmail({
          totalFlagged,
          autoHidden,
          reviewUrl,
          highlights,
          locale,
        });
        const html = await render(element, { pretty: true });
        const text = await render(element, { plainText: true });
        const subject = t("subject");

        callback(null, { subject, html, text });
      } catch (err) {
        callback(
          {
            code: grpc.status.INTERNAL,
            message: (err as Error).message,
          } as any,
          null,
        );
      }
    },
    ComposeDayShiftSkip: async (
      call: grpc.ServerUnaryCall<any, any>,
      callback: grpc.sendUnaryData<any>,
    ) => {
      try {
        const locale = (call.request?.locale as string) || "ja";
        const todayDate = (call.request?.today_date as string) || "";
        const todayNames = (call.request?.today_names as string) || "";
        const nextDate = (call.request?.next_date as string) || "";
        const nextNames = (call.request?.next_names as string) || "";

        const { t } = await getT("dayshift-skip", locale);
        const element = await DayShiftSkipEmail({
          todayDate,
          todayNames,
          nextDate,
          nextNames,
          locale,
        });
        const html = await render(element, { pretty: true });
        const text = await render(element, { plainText: true });
        const subject = t("subject");

        callback(null, { subject, html, text });
      } catch (err) {
        callback(
          {
            code: grpc.status.INTERNAL,
            message: (err as Error).message,
          } as any,
          null,
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
      },
    );
  });
  server.start();
}
