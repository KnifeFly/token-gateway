export interface PaginationProps {
  className?: string;
  limit: number;
  limitOptions?: number[];
  nextDisabled?: boolean;
  onLimitChange: (limit: number) => void;
  onNext?: () => void;
  onPrevious?: () => void;
  previousDisabled?: boolean;
  summary?: string;
}

export function Pagination({
  className = "",
  limit,
  limitOptions = [10, 20, 50],
  nextDisabled = true,
  onLimitChange,
  onNext,
  onPrevious,
  previousDisabled = true,
  summary
}: PaginationProps) {
  return (
    <div className={`tg-pagination ${className}`.trim()}>
      {summary ? <span className="tg-pagination-summary">{summary}</span> : null}
      <label>
        Limit
        <select value={limit} onChange={(event) => onLimitChange(Number(event.target.value))}>
          {limitOptions.map((value) => (
            <option key={value} value={value}>
              {value}
            </option>
          ))}
        </select>
      </label>
      {onPrevious ? (
        <button disabled={previousDisabled} onClick={onPrevious} type="button">
          上一页
        </button>
      ) : null}
      {onNext ? (
        <button disabled={nextDisabled} onClick={onNext} type="button">
          下一页
        </button>
      ) : null}
    </div>
  );
}
