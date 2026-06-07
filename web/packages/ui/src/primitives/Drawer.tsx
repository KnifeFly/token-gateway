import type { ReactNode } from "react";

export interface DrawerProps {
  children: ReactNode;
  className?: string;
  open: boolean;
  title: ReactNode;
}

export function Drawer({ children, className = "", open, title }: DrawerProps) {
  if (!open) {
    return null;
  }

  return (
    <aside aria-label={typeof title === "string" ? title : undefined} className={`tg-drawer ${className}`.trim()}>
      <div className="tg-drawer-heading">
        <strong>{title}</strong>
      </div>
      <div className="tg-drawer-body">{children}</div>
    </aside>
  );
}
