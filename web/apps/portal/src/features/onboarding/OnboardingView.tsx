import { StatusBadge } from "@token-gateway/ui";

import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { Onboarding } from "../../shared/types";

export function OnboardingView({ onboarding }: { onboarding?: Onboarding }) {
  return (
    <section className="panel">
      <PanelHeading title="Onboarding" meta={onboarding?.generated_at} />
      <div className="step-list">
        {(onboarding?.steps ?? []).map((step) => (
          <div className="step-row" key={step.id}>
            <StatusBadge tone={step.complete ? "success" : "warning"}>
              {step.complete ? "Done" : "Open"}
            </StatusBadge>
            <strong>{step.title}</strong>
          </div>
        ))}
      </div>
    </section>
  );
}
