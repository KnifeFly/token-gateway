import { navItems } from "../../app/routes";
import { portalCopy } from "../../shared/i18n";
import { PanelHeading } from "../../shared/layout/PanelHeading";

const playgroundRoute = navItems.find((item) => item.id === "playground");

export function PlaygroundView() {
  return (
    <section className="panel-grid split">
      <article className="panel">
        <PanelHeading title={portalCopy.playground.title} />
        <p className="panel-note">{portalCopy.playground.description}</p>
        <div className="table route-table" role="table">
          <div className="table-row table-head route" role="row">
            <span role="columnheader">{portalCopy.playground.routeColumn}</span>
            <span role="columnheader">{portalCopy.playground.purposeColumn}</span>
            <span role="columnheader">{portalCopy.playground.nextTaskColumn}</span>
          </div>
          <div className="table-row route" role="row">
            <span role="cell">{playgroundRoute?.route}</span>
            <span role="cell">{playgroundRoute?.purpose}</span>
            <span role="cell">{playgroundRoute?.nextTask}</span>
          </div>
        </div>
      </article>
      <article className="panel">
        <PanelHeading title={portalCopy.playground.guardrailTitle} />
        <div className="tag-list">
          {portalCopy.playground.guardrailItems.map((item) => (
            <span key={item}>{item}</span>
          ))}
        </div>
      </article>
    </section>
  );
}
