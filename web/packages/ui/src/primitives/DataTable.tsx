import type { ReactNode } from "react";

export interface DataTableColumn<TRow> {
  key: string;
  header: ReactNode;
  render: (row: TRow) => ReactNode;
}

export interface DataTableProps<TRow> {
  ariaLabel: string;
  className?: string;
  columns: Array<DataTableColumn<TRow>>;
  empty?: ReactNode;
  getRowKey: (row: TRow) => string;
  rows: TRow[];
  rowClassName?: string;
}

export function DataTable<TRow>({
  ariaLabel,
  className = "",
  columns,
  empty,
  getRowKey,
  rows,
  rowClassName = "table-row"
}: DataTableProps<TRow>) {
  return (
    <div className={`table tg-data-table ${className}`.trim()} role="table" aria-label={ariaLabel}>
      <div className={`${rowClassName} table-head`} role="row">
        {columns.map((column) => (
          <span key={column.key} role="columnheader">
            {column.header}
          </span>
        ))}
      </div>
      {rows.length > 0
        ? rows.map((row) => (
            <div className={rowClassName} key={getRowKey(row)} role="row">
              {columns.map((column) => (
                <span key={column.key} role="cell">
                  {column.render(row)}
                </span>
              ))}
            </div>
          ))
        : null}
      {rows.length === 0 && empty ? <div className="tg-table-empty">{empty}</div> : null}
    </div>
  );
}
