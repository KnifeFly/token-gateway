import { Button } from "@token-gateway/ui";
import { formatInteger } from "@token-gateway/format";
import type { FormEvent } from "react";

import { formatMaybeDate } from "../../shared/format";
import { enabledLabel, portalCopy } from "../../shared/i18n";
import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { APIKey, APIKeyCreateResponse, APIKeyRotateResponse } from "../../shared/types";

function listLabel(values: string[] | undefined, fallback: string): string {
  return values && values.length > 0 ? values.join(", ") : fallback;
}

function expiresLabel(value?: string | null): string {
  return value ? formatMaybeDate(value) : portalCopy.apiKeys.neverExpires;
}

function usageLabel(key: APIKey): string {
  return portalCopy.apiKeys.usageRequests(formatInteger(key.usage_summary?.requests ?? 0));
}

export function APIKeysView({
  apiKeys,
  createdKey,
  currentKeyID,
  derivedExpiresAt,
  derivedIPAllowlist,
  derivedModels,
  derivedName,
  onCreateKey,
  onDisableKey,
  onRotateKey,
  setDerivedExpiresAt,
  setDerivedIPAllowlist,
  setDerivedModels,
  setDerivedName
}: {
  apiKeys: APIKey[];
  createdKey?: APIKeyCreateResponse | APIKeyRotateResponse;
  currentKeyID: string;
  derivedExpiresAt: string;
  derivedIPAllowlist: string;
  derivedModels: string;
  derivedName: string;
  onCreateKey(event: FormEvent<HTMLFormElement>): void;
  onDisableKey(keyID: string): void;
  onRotateKey(keyID: string): void;
  setDerivedExpiresAt(value: string): void;
  setDerivedIPAllowlist(value: string): void;
  setDerivedModels(value: string): void;
  setDerivedName(value: string): void;
}) {
  return (
    <section className="panel-grid split">
      <article className="panel">
        <PanelHeading title={portalCopy.apiKeys.title} />
        <div className="table" role="table">
          <div className="table-row table-head keys" role="row">
            <span role="columnheader">{portalCopy.apiKeys.nameColumn}</span>
            <span role="columnheader">{portalCopy.apiKeys.modelsColumn}</span>
            <span role="columnheader">{portalCopy.apiKeys.ipColumn}</span>
            <span role="columnheader">{portalCopy.apiKeys.expiresColumn}</span>
            <span role="columnheader">{portalCopy.apiKeys.usageColumn}</span>
            <span role="columnheader">{portalCopy.apiKeys.stateColumn}</span>
            <span role="columnheader">{portalCopy.apiKeys.actionColumn}</span>
          </div>
          {apiKeys.map((key) => (
            <div className="table-row keys" key={key.id} role="row">
              <span role="cell">{key.name}</span>
              <span role="cell">{listLabel(key.allowed_models, portalCopy.apiKeys.noLimit)}</span>
              <span role="cell">{listLabel(key.ip_allowlist, portalCopy.apiKeys.noLimit)}</span>
              <span role="cell">{expiresLabel(key.expires_at)}</span>
              <span role="cell">{usageLabel(key)}</span>
              <span role="cell">{enabledLabel(key.enabled)}</span>
              <span role="cell">
                <span className="inline-actions">
                  <Button
                    disabled={!key.enabled}
                    onClick={() => onRotateKey(key.id)}
                    variant="ghost"
                  >
                    {portalCopy.apiKeys.rotateAction}
                  </Button>
                  <Button
                    disabled={!key.enabled || key.id === currentKeyID}
                    onClick={() => onDisableKey(key.id)}
                    variant="ghost"
                  >
                    {portalCopy.apiKeys.disableAction}
                  </Button>
                </span>
              </span>
            </div>
          ))}
        </div>
      </article>
      <article className="panel">
        <PanelHeading title={portalCopy.apiKeys.createTitle} />
        <form className="stacked-form" onSubmit={onCreateKey}>
          <label className="field">
            <span>{portalCopy.apiKeys.nameLabel}</span>
            <input onChange={(event) => setDerivedName(event.target.value)} value={derivedName} />
          </label>
          <label className="field">
            <span>{portalCopy.apiKeys.allowedModelsLabel}</span>
            <input
              onChange={(event) => setDerivedModels(event.target.value)}
              placeholder="gpt-public"
              value={derivedModels}
            />
          </label>
          <label className="field">
            <span>{portalCopy.apiKeys.ipAllowlistLabel}</span>
            <input
              onChange={(event) => setDerivedIPAllowlist(event.target.value)}
              placeholder="203.0.113.10, 2001:db8::/32"
              value={derivedIPAllowlist}
            />
          </label>
          <label className="field">
            <span>{portalCopy.apiKeys.expiresAtLabel}</span>
            <input
              onChange={(event) => setDerivedExpiresAt(event.target.value)}
              type="datetime-local"
              value={derivedExpiresAt}
            />
          </label>
          <Button type="submit" variant="primary">
            {portalCopy.apiKeys.createAction}
          </Button>
        </form>
        {createdKey ? (
          <div className="secret-box">
            <span>{createdKey.api_key.name}</span>
            <small>{portalCopy.apiKeys.plaintextHint}</small>
            <code>{createdKey.plaintext_key}</code>
          </div>
        ) : null}
      </article>
    </section>
  );
}
