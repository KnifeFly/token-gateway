import type { ReactNode } from "react";

export interface EmptyStateProps {
  children?: ReactNode;
  title: string;
}

export function EmptyState({ children, title }: EmptyStateProps) {
  return (
    <div className="tg-empty-state">
      <strong>{title}</strong>
      {children ? <span>{children}</span> : null}
    </div>
  );
}

export interface LoadingStateProps {
  label?: string;
}

export function LoadingState({ label = "加载中" }: LoadingStateProps) {
  return (
    <div className="tg-loading-state" role="status">
      <span className="tg-loading-dot" aria-hidden="true" />
      <Skeleton width="5rem" />
      {label}
    </div>
  );
}

export interface SkeletonProps {
  className?: string;
  width?: string;
}

export function Skeleton({ className = "", width }: SkeletonProps) {
  return <span aria-hidden="true" className={`tg-skeleton ${className}`.trim()} style={width ? { width } : undefined} />;
}
