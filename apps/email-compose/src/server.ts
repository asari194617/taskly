import path from "node:path";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";
import { render } from "@react-email/render";
import { LoginCodeEmail } from "./templates/LoginCode.js";
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
