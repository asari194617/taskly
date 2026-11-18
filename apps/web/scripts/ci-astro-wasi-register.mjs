import { registerHooks } from "node:module";

// The primary CI VM exposes an x86 CPU without AVX. Its Astro native binding
// crashes with SIGILL before its own WASI fallback can run. Rolldown and
// Satteri select their WASI builds through NAPI_RS_FORCE_WASI.
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "@astrojs/compiler-binding") {
      return nextResolve("@astrojs/compiler-binding-wasm32-wasi", context);
    }
    return nextResolve(specifier, context);
  },
});
