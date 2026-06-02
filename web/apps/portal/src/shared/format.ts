import { ConsoleAPIError } from "@token-gateway/api-client";
import { formatISODateTime } from "@token-gateway/format";

import type { CreditsResponse, TaskObject } from "./types";

export function primaryCreditBucket(credits?: CreditsResponse) {
  return credits?.data.token ?? Object.values(credits?.data ?? {})[0];
}

export function moneyValue(value = 0): string {
  return new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 4 }).format(value);
}

export function formatMaybeDate(value?: string): string {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return formatISODateTime(date);
}

export function splitModels(value: string): string[] {
  return value
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean);
}

export function taskString(task: TaskObject, key: string): string {
  const value = task[key];
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number") {
    return String(value);
  }
  return "";
}

export function errorMessage(error: unknown): string {
  if (error instanceof ConsoleAPIError) {
    return `${error.code}: ${error.message}`;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return "请求失败";
}
