import type { ReactNode } from "react";

import { Button } from "./Button";

export interface ConfirmDialogProps {
  busy?: boolean;
  cancelLabel?: string;
  children?: ReactNode;
  className?: string;
  confirmDisabled?: boolean;
  confirmLabel?: string;
  description?: ReactNode;
  onCancel?: () => void;
  onConfirm: () => void;
  open: boolean;
  title: ReactNode;
}

export function ConfirmDialog({
  busy = false,
  cancelLabel = "取消",
  children,
  className = "",
  confirmDisabled = false,
  confirmLabel = "确认",
  description,
  onCancel,
  onConfirm,
  open,
  title
}: ConfirmDialogProps) {
  if (!open) {
    return null;
  }

  return (
    <div aria-modal="false" className={`tg-confirm-dialog ${className}`.trim()} role="dialog">
      <strong>{title}</strong>
      {description ? <span>{description}</span> : null}
      {children}
      <div className="tg-confirm-dialog-actions">
        {onCancel ? (
          <Button disabled={busy} onClick={onCancel} variant="ghost">
            {cancelLabel}
          </Button>
        ) : null}
        <Button disabled={confirmDisabled || busy} onClick={onConfirm} variant="primary">
          {busy ? "处理中" : confirmLabel}
        </Button>
      </div>
    </div>
  );
}
