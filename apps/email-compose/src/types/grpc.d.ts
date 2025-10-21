declare module "@grpc/proto-loader" {
  import { PackageDefinition } from "@grpc/grpc-js";
  export function loadSync(filename: string, options?: any): PackageDefinition;
}
