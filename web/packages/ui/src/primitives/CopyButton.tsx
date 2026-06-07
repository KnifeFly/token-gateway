import { useState } from "react";

export interface CopyButtonProps {
  label?: string;
  value: string;
}

export function CopyButton({ label = "复制", value }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);

  async function copyValue() {
    if (!value) {
      return;
    }
    await navigator.clipboard?.writeText(value);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1400);
  }

  return (
    <button className="tg-copy-button" disabled={!value} onClick={copyValue} type="button">
      {copied ? "已复制" : label}
    </button>
  );
}
