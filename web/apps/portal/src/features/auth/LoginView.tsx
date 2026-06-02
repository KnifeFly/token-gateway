import { Button } from "@token-gateway/ui";
import type { FormEvent } from "react";

import { portalCopy } from "../../shared/i18n";

export function LoginView({
  apiKey,
  busy,
  message,
  onAPIKeyChange,
  onSubmit
}: {
  apiKey: string;
  busy: boolean;
  message: string;
  onAPIKeyChange(value: string): void;
  onSubmit(event: FormEvent<HTMLFormElement>): void;
}) {
  return (
    <main className="login-shell">
      <form className="login-panel" onSubmit={onSubmit}>
        <div className="brand login-brand">
          <span className="brand-mark">TG</span>
          <span>{portalCopy.brand}</span>
        </div>
        <label className="field">
          <span>{portalCopy.login.apiKeyLabel}</span>
          <input
            autoComplete="off"
            name="api_key"
            onChange={(event) => onAPIKeyChange(event.target.value)}
            placeholder="tg-..."
            type="password"
            value={apiKey}
          />
        </label>
        {message ? <div className="error-banner">{message}</div> : null}
        <Button disabled={busy || apiKey.trim() === ""} type="submit" variant="primary">
          {busy ? portalCopy.login.signingIn : portalCopy.login.signIn}
        </Button>
      </form>
    </main>
  );
}
