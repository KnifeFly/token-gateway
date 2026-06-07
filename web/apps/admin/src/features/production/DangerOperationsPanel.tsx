import { Button, StatusBadge } from "@token-gateway/ui";
import { useMemo, useState } from "react";

import type { ProductionData } from "./productionApi";
import {
  publishSnapshot,
  replaySettlement,
  retryCallback,
  rollbackSnapshot,
  validateSnapshot
} from "./productionApi";

interface DangerOperationsPanelProps {
  csrfToken: string;
  data?: ProductionData;
  onCompleted: () => Promise<void>;
}

type OperationKey = "validate" | "publish" | "rollback" | "replay-settlement" | "retry-callback";

const operations: Array<{
  key: OperationKey;
  label: string;
  description: string;
  requiresTarget?: "settlement" | "callback";
}> = [
  {
    key: "validate",
    label: "Validate snapshot",
    description: "校验当前配置图，不切换 active snapshot。"
  },
  {
    key: "publish",
    label: "Publish snapshot",
    description: "发布 active runtime snapshot，影响新请求热路径。"
  },
  {
    key: "rollback",
    label: "Rollback snapshot",
    description: "回滚到 previous runtime snapshot。"
  },
  {
    key: "replay-settlement",
    label: "Replay settlement",
    description: "重放一个待修复 failed settlement。",
    requiresTarget: "settlement"
  },
  {
    key: "retry-callback",
    label: "Retry callback",
    description: "重试一个 due callback outbox 事件。",
    requiresTarget: "callback"
  }
];

function firstID(rows: Array<Record<string, unknown>>, fields: string[]): string {
  for (const row of rows) {
    for (const field of fields) {
      const value = row[field];
      if (typeof value === "string" && value.trim()) {
        return value;
      }
    }
  }
  return "";
}

function resultSummary(value: unknown): string {
  if (value === undefined) {
    return "done";
  }
  return JSON.stringify(value, null, 2);
}

export function DangerOperationsPanel({ csrfToken, data, onCompleted }: DangerOperationsPanelProps) {
  const [operation, setOperation] = useState<OperationKey>("validate");
  const [reason, setReason] = useState("P22 console production verification");
  const [targetID, setTargetID] = useState("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");

  const suggestedTarget = useMemo(() => {
    if (operation === "replay-settlement") {
      return firstID(data?.settlements.data ?? [], ["id", "settlement_id", "request_id"]);
    }
    if (operation === "retry-callback") {
      return firstID(data?.callbacks.data ?? [], ["id", "callback_id", "task_id"]);
    }
    return "";
  }, [data?.callbacks.data, data?.settlements.data, operation]);

  const selected = operations.find((item) => item.key === operation) ?? operations[0];
  const effectiveTarget = targetID.trim() || suggestedTarget;
  const canRun = reason.trim() !== "" && csrfToken !== "" && (!selected.requiresTarget || effectiveTarget !== "");

  async function runOperation() {
    setBusy(true);
    setMessage("");
    const mutation = { csrfToken, reason: reason.trim() };
    try {
      let result: unknown;
      if (operation === "validate") {
        result = await validateSnapshot(mutation);
      } else if (operation === "publish") {
        result = await publishSnapshot(mutation);
      } else if (operation === "rollback") {
        result = await rollbackSnapshot(mutation);
      } else if (operation === "replay-settlement") {
        result = await replaySettlement(effectiveTarget, mutation);
      } else {
        result = await retryCallback(effectiveTarget, mutation);
      }
      setMessage(resultSummary(result));
      await onCompleted();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="panel danger-panel" id="settings">
      <div className="panel-heading">
        <div>
          <h2>危险操作与权限体验</h2>
          <p>所有 mutation 都通过 `/api/admin/v1/*`，并强制 CSRF、理由、幂等键和审计。</p>
        </div>
        <StatusBadge tone="warning">P22-T02</StatusBadge>
      </div>

      <div className="danger-layout">
        <label>
          操作
          <select value={operation} onChange={(event) => setOperation(event.target.value as OperationKey)}>
            {operations.map((item) => (
              <option key={item.key} value={item.key}>
                {item.label}
              </option>
            ))}
          </select>
        </label>

        <label>
          Reason / X-Reason
          <input value={reason} onChange={(event) => setReason(event.target.value)} />
        </label>

        {selected.requiresTarget ? (
          <label>
            Target ID
            <input
              placeholder={suggestedTarget || "没有可用目标"}
              value={targetID}
              onChange={(event) => setTargetID(event.target.value)}
            />
          </label>
        ) : null}

        <div className="danger-confirm">
          <strong>{selected.label}</strong>
          <span>{selected.description}</span>
          {selected.requiresTarget ? <code>{effectiveTarget || "missing target"}</code> : null}
          <Button disabled={!canRun || busy} onClick={runOperation} variant="primary">
            {busy ? "执行中" : "确认执行"}
          </Button>
        </div>
      </div>

      {message ? <pre className="operation-result">{message}</pre> : null}
    </section>
  );
}
