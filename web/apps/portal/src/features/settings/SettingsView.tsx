import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { ProjectSettings, Session } from "../../shared/types";

export function SettingsView({ session, settings }: { session: Session; settings?: ProjectSettings }) {
  return (
    <section className="panel">
      <PanelHeading title="Project" meta={settings?.generated_at} />
      <dl className="settings-list">
        <div>
          <dt>Tenant</dt>
          <dd>{settings?.tenant_id ?? session.tenant_id}</dd>
        </div>
        <div>
          <dt>Project</dt>
          <dd>{settings?.project_id ?? session.project_id}</dd>
        </div>
        <div>
          <dt>API key</dt>
          <dd>{settings?.api_key_id ?? session.api_key_id}</dd>
        </div>
        <div>
          <dt>Allowed models</dt>
          <dd>{(settings?.allowed_models ?? session.allowed_models ?? []).join(", ")}</dd>
        </div>
      </dl>
    </section>
  );
}
