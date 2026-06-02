import { formatMaybeDate } from "../format";

export function PanelHeading({ meta, title }: { meta?: string; title: string }) {
  return (
    <div className="panel-heading">
      <h2>{title}</h2>
      {meta ? <span>{formatMaybeDate(meta)}</span> : null}
    </div>
  );
}
