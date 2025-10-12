/// <reference path="../.astro/types.d.ts" />
/// <reference types="react" />
/// <reference types="react-dom" />

interface ImportMetaEnv {
  readonly PUBLIC_API_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

// Allow importing global CSS in .astro files
declare module "*.css";
