import { portalCopy } from "../../shared/i18n";
import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { ProjectSettings, Session } from "../../shared/types";

export function SettingsView({ session, settings }: { session: Session; settings?: ProjectSettings }) {
  return (
    <section className="panel">
      <PanelHeading title={portalCopy.settings.title} meta={settings?.generated_at} />
      <dl className="settings-list">
        <div>
          <dt>{portalCopy.settings.tenant}</dt>
          <dd>{settings?.tenant_id ?? session.tenant_id}</dd>
        </div>
        <div>
          <dt>{portalCopy.settings.project}</dt>
          <dd>{settings?.project_id ?? session.project_id}</dd>
        </div>
        <div>
          <dt>{portalCopy.settings.apiKey}</dt>
          <dd>{settings?.api_key_id ?? session.api_key_id}</dd>
        </div>
        <div>
          <dt>{portalCopy.settings.allowedModels}</dt>
          <dd>{(settings?.allowed_models ?? session.allowed_models ?? []).join(", ")}</dd>
        </div>
      </dl>
    </section>
  );
}
