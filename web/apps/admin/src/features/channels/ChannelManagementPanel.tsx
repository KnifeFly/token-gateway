import { Button, DataTable, EmptyState, LoadingState, StatusBadge } from "@token-gateway/ui";
import { useEffect, useMemo, useState } from "react";

import { adminCopy } from "../../shared/i18n";
import {
  listAdminChannelHealthEvents,
  listAdminChannels,
  testAdminChannel,
  type AdminChannelHealthEvent,
  type AdminChannelView
} from "./channelApi";

interface ChannelManagementPanelProps {
  csrfToken: string;
}

const workflowRows = [
  { label: adminCopy.channels.actions.create, endpoint: "POST /api/admin/v1/channels" },
  {
    label: adminCopy.channels.actions.rotate,
    endpoint: "POST /api/admin/v1/channels/{id}/rotate-credential"
  },
  { label: adminCopy.channels.actions.test, endpoint: "POST /api/admin/v1/channels/{id}/test" },
  {
    label: adminCopy.channels.actions.syncPreview,
    endpoint: "POST /api/admin/v1/channels/{id}/sync-preview"
  },
  {
    label: adminCopy.channels.actions.syncApply,
    endpoint: "POST /api/admin/v1/channels/{id}/sync-apply"
  },
  {
    label: adminCopy.channels.actions.healthEvents,
    endpoint: "GET /api/admin/v1/channels/{id}/health-events"
  }
];

function channelStatusLabel(value?: string): string {
  const labels: Record<string, string> = {
    healthy: adminCopy.channels.state.healthy,
    unknown: adminCopy.channels.state.unknown,
    configured: adminCopy.channels.state.configured,
    passed: adminCopy.channels.state.passed,
    untested: adminCopy.channels.state.untested,
    incomplete: adminCopy.channels.state.incomplete,
    disabled: adminCopy.channels.state.disabled
  };
  return labels[(value ?? "").toLowerCase()] ?? value ?? adminCopy.channels.state.unknown;
}

function describeError(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function ChannelManagementPanel({ csrfToken }: ChannelManagementPanelProps) {
  const [channels, setChannels] = useState<AdminChannelView[]>([]);
  const [selectedID, setSelectedID] = useState("");
  const [healthEvents, setHealthEvents] = useState<AdminChannelHealthEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");

  async function loadChannels() {
    setLoading(true);
    setMessage("");
    try {
      const response = await listAdminChannels();
      setChannels(response.data);
      setSelectedID((current) => current || response.data[0]?.id || "");
    } catch (error) {
      setMessage(describeError(error));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadChannels();
  }, []);

  const selected = useMemo(
    () => channels.find((channel) => channel.id === selectedID) ?? channels[0],
    [channels, selectedID]
  );

  useEffect(() => {
    if (!selected?.id) {
      setHealthEvents([]);
      return;
    }
    let active = true;
    async function loadHealth() {
      try {
        const response = await listAdminChannelHealthEvents(selected.id);
        if (active) {
          setHealthEvents(response.data);
        }
      } catch {
        if (active) {
          setHealthEvents([]);
        }
      }
    }
    void loadHealth();
    return () => {
      active = false;
    };
  }, [selected?.id]);

  async function runChannelTest(channelID: string) {
    setBusy(true);
    setMessage("");
    try {
      const result = await testAdminChannel(channelID, {
        csrfToken,
        reason: `P25 Admin channel test ${channelID}`
      });
      setMessage(`${result.status}: ${result.message}`);
      await loadChannels();
    } catch (error) {
      setMessage(describeError(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="panel channel-panel" id="routing">
      <div className="panel-heading">
        <div>
          <h2>{adminCopy.channels.title}</h2>
          <p>{adminCopy.channels.subtitle}</p>
        </div>
        <StatusBadge tone="success">BFF connected</StatusBadge>
      </div>

      {message ? <div className="inline-alert">{message}</div> : null}
      {loading ? <LoadingState label="正在加载渠道" /> : null}

      <div className="channel-layout">
        <article className="channel-section">
          <h3>{adminCopy.channels.sections.workflow}</h3>
          <DataTable
            ariaLabel={adminCopy.channels.sections.workflow}
            className="channel-table"
            columns={[
              {
                key: "channel",
                header: adminCopy.channels.columns.channel,
                render: (channel) => (
                  <>
                    <strong>{channel.id}</strong>
                    <small>{channel.base_url}</small>
                  </>
                )
              },
              { key: "provider", header: adminCopy.channels.columns.provider, render: (channel) => channel.provider_type },
              {
                key: "state",
                header: adminCopy.channels.columns.state,
                render: (channel) =>
                  channel.enabled ? adminCopy.channels.state.enabled : adminCopy.channels.state.disabled
              },
              {
                key: "credential",
                header: adminCopy.channels.columns.credential,
                render: (channel) =>
                  channel.credential_configured
                    ? adminCopy.channels.state.configured
                    : adminCopy.channels.state.missing
              },
              { key: "models", header: adminCopy.channels.columns.models, render: (channel) => channel.model_count },
              {
                key: "action",
                header: adminCopy.channels.columns.action,
                render: (channel) => (
                  <span className="inline-actions">
                    <Button disabled={busy || !csrfToken} onClick={() => runChannelTest(channel.id)} variant="ghost">
                      {adminCopy.channels.actions.test}
                    </Button>
                    <Button onClick={() => setSelectedID(channel.id)} variant="ghost">
                      详情
                    </Button>
                  </span>
                )
              }
            ]}
            empty={
              <EmptyState title="暂无渠道">
                当前 BFF 没有返回渠道，检查 seed fixture 或控制面配置。
              </EmptyState>
            }
            getRowKey={(channel) => channel.id}
            rowClassName="table-row channel-row"
            rows={channels}
          />
        </article>

        <article className="channel-section">
          <h3>{adminCopy.channels.sections.coverage}</h3>
          <div className="coverage-grid">
            {(selected?.models ?? []).map((model) => (
              <div className="coverage-item" key={`${selected?.id}-${model.public_model}`}>
                <strong>{model.public_model}</strong>
                <span>{model.upstream_model}</span>
                <small>
                  {adminCopy.channels.columns.health} {channelStatusLabel(model.health_status)} ·{" "}
                  {adminCopy.channels.columns.cost} {channelStatusLabel(model.cost_config_status)}
                </small>
              </div>
            ))}
            {(selected?.models ?? []).length === 0 ? (
              <EmptyState title="暂无模型覆盖">选择的渠道没有返回模型覆盖。</EmptyState>
            ) : null}
          </div>
        </article>

        <article className="channel-section">
          <h3>{adminCopy.channels.sections.routeHints}</h3>
          <div className="coverage-grid compact">
            {(selected?.route_policy_hints ?? []).map((hint) => (
              <div className="coverage-item" key={`${selected?.id}-${hint.route_id}`}>
                <strong>{hint.public_model}</strong>
                <span>{hint.route_id}</span>
                <small>
                  优先级 {hint.priority} · 权重 {hint.weight}
                </small>
              </div>
            ))}
            {(selected?.route_policy_hints ?? []).length === 0 ? (
              <EmptyState title="暂无路由提示">当前渠道没有 route policy hint。</EmptyState>
            ) : null}
          </div>
        </article>

        <article className="channel-section">
          <h3>{adminCopy.channels.actions.healthEvents}</h3>
          <div className="coverage-grid compact">
            {healthEvents.map((event) => (
              <div className="coverage-item" key={`${event.channel_id}-${event.status}-${event.observed_at}`}>
                <strong>{event.status}</strong>
                <span>{event.message}</span>
                <small>{event.observed_at}</small>
              </div>
            ))}
            {healthEvents.length === 0 ? <EmptyState title="暂无健康事件">BFF 未返回健康事件。</EmptyState> : null}
          </div>
        </article>

        <article className="guardrail-band channel-guardrail">
          <h3>{adminCopy.channels.sections.safeFields}</h3>
          <div className="tag-list">
            <span>{adminCopy.channels.hints.noSecret}</span>
            <span>{adminCopy.channels.hints.noGroup}</span>
            <span>{adminCopy.channels.hints.audit}</span>
          </div>
          <div className="endpoint-list">
            {workflowRows.map((action) => (
              <code key={action.endpoint}>{action.endpoint}</code>
            ))}
          </div>
        </article>
      </div>
    </section>
  );
}
