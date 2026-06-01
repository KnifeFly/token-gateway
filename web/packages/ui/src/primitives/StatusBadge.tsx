export interface StatusBadgeProps {
  tone?: "neutral" | "success" | "warning";
  children: string;
}

export function StatusBadge({ children, tone = "neutral" }: StatusBadgeProps) {
  return <span className={`tg-status tg-status--${tone}`}>{children}</span>;
}
