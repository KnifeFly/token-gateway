import type { ReactNode } from "react";

export interface FilterBarProps {
  children: ReactNode;
  className?: string;
}

export function FilterBar({ children, className = "" }: FilterBarProps) {
  return <div className={`filter-bar tg-filter-bar ${className}`.trim()}>{children}</div>;
}
