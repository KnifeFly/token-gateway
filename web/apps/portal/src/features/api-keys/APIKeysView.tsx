import { Button } from "@token-gateway/ui";
import type { FormEvent } from "react";

import { enabledLabel, portalCopy } from "../../shared/i18n";
import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { APIKey, APIKeyCreateResponse } from "../../shared/types";

export function APIKeysView({
  apiKeys,
  createdKey,
  currentKeyID,
  derivedModels,
  derivedName,
  onCreateKey,
  onDisableKey,
  setDerivedModels,
  setDerivedName
}: {
  apiKeys: APIKey[];
  createdKey?: APIKeyCreateResponse;
  currentKeyID: string;
  derivedModels: string;
  derivedName: string;
  onCreateKey(event: FormEvent<HTMLFormElement>): void;
  onDisableKey(keyID: string): void;
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
            <span role="columnheader">{portalCopy.apiKeys.stateColumn}</span>
            <span role="columnheader">{portalCopy.apiKeys.actionColumn}</span>
          </div>
          {apiKeys.map((key) => (
            <div className="table-row keys" key={key.id} role="row">
              <span role="cell">{key.name}</span>
              <span role="cell">{key.allowed_models.join(", ")}</span>
              <span role="cell">{enabledLabel(key.enabled)}</span>
              <span role="cell">
                <Button
                  disabled={!key.enabled || key.id === currentKeyID}
                  onClick={() => onDisableKey(key.id)}
                  variant="ghost"
                >
                  {portalCopy.apiKeys.disableAction}
                </Button>
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
