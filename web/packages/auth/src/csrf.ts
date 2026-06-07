export function readCSRFToken(metaName = "csrf-token"): string | undefined {
  if (typeof document === "undefined") {
    return undefined;
  }
  return document.querySelector<HTMLMetaElement>(`meta[name="${metaName}"]`)?.content;
}
