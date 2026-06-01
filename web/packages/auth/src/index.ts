export interface BrowserSession {
  authenticated: boolean;
  subject?: string;
  expiresAt?: string;
}

export function sessionStateLabel(session: BrowserSession): "Signed in" | "Signed out" {
  return session.authenticated ? "Signed in" : "Signed out";
}

export function readCSRFToken(metaName = "csrf-token"): string | undefined {
  if (typeof document === "undefined") {
    return undefined;
  }
  return document.querySelector<HTMLMetaElement>(`meta[name="${metaName}"]`)?.content;
}
