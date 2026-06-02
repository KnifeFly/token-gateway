import { formatISODateTime } from "@token-gateway/format";
import { StatusBadge } from "@token-gateway/ui";

import { adminCutScopeItems, adminRouteSections } from "../../app/routes";
import { adminCopy } from "../../shared/i18n";
import { scaffoldSession } from "../../shared/session/session";

const implementedRouteAnchors = new Set(["catalog", "routing", "accounts"]);

export function RoutePlanPanel() {
  return (
    <section className="panel route-plan">
      <div className="panel-heading">
        <div>
          <h2>{adminCopy.routePlan.title}</h2>
          <p>
            {adminCopy.routePlan.generatedAtLabel}：
            {formatISODateTime(scaffoldSession.expiresAt)}
          </p>
        </div>
        <StatusBadge tone="warning">{adminCopy.routePlan.status}</StatusBadge>
      </div>

      <div className="route-grid" aria-label={adminCopy.routePlan.title}>
        {adminRouteSections.map((section) => (
          <article
            className="route-card"
            id={implementedRouteAnchors.has(section.id) ? `plan-${section.id}` : section.id}
            key={section.id}
          >
            <div className="route-card-header">
              <h3>{section.label}</h3>
              <span>{section.routePrefix}</span>
            </div>
            <p>{section.purpose}</p>
            <dl className="route-meta">
              <div>
                <dt>{adminCopy.routePlan.ownerColumn}</dt>
                <dd>{section.owner}</dd>
              </div>
              <div>
                <dt>{adminCopy.routePlan.nextTaskColumn}</dt>
                <dd>{section.nextTask}</dd>
              </div>
            </dl>
            <div className="tag-list" aria-label={`${section.label}${adminCopy.routePlan.pagesLabel}`}>
              {section.pages.map((page) => (
                <span key={page}>{page}</span>
              ))}
            </div>
          </article>
        ))}
      </div>

      <section className="guardrail-band" aria-label={adminCopy.routePlan.guardrailTitle}>
        <div>
          <h3>{adminCopy.routePlan.guardrailTitle}</h3>
          <p>{adminCopy.routePlan.guardrailDescription}</p>
        </div>
        <div className="tag-list danger">
          {adminCutScopeItems.map((item) => (
            <span key={item}>{item}</span>
          ))}
        </div>
      </section>
    </section>
  );
}
