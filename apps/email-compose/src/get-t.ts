import { i18next, initI18n } from "./i18n.js";

export async function getT(namespace?: string | string[], locale?: string) {
  await initI18n();
  if (locale && i18next.resolvedLanguage !== locale) {
    await i18next.changeLanguage(locale);
  }
  if (namespace) {
    const namespaces = Array.isArray(namespace) ? namespace : [namespace];
    for (const ns of namespaces) {
      if (!i18next.hasLoadedNamespace(ns)) {
        await i18next.loadNamespaces(ns);
      }
    }
  }
  return {
    t: i18next.getFixedT(
      locale ?? i18next.resolvedLanguage ?? i18next.language,
      Array.isArray(namespace) ? namespace[0] : namespace,
    ),
    i18n: i18next,
  };
}
