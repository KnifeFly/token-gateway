import type { ReactNode } from "react";

export interface ToastProps {
  children: ReactNode;
  className?: string;
  tone?: "danger" | "info" | "success" | "warning";
}

export function Toast({ children, className = "", tone = "info" }: ToastProps) {
  return (
    <div className={`tg-toast tg-toast--${tone} ${className}`.trim()} role="status">
      {children}
    </div>
  );
}
