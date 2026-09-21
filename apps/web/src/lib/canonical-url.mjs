const productionSite = new URL("https://taskly.iy0306.com");

/**
 * Build a stable page URL without carrying request-specific query parameters
 * (for example, an email address in an authentication flow) into metadata.
 */
export function buildCanonicalUrl(pageUrl, configuredSite = productionSite) {
  const pathname = pageUrl.pathname.endsWith("/")
    ? pageUrl.pathname
    : `${pageUrl.pathname}/`;
  const canonical = new URL(configuredSite);
  canonical.pathname = pathname;
  canonical.search = "";
  canonical.hash = "";
  return canonical.toString();
}
