export interface BrowserSession {
  authenticated: boolean;
  subject?: string;
  expiresAt?: string;
}

export function sessionStateLabel(session: BrowserSession): "Signed in" | "Signed out" {
  return session.authenticated ? "Signed in" : "Signed out";
}
