export function defaultConsoleBaseURL(): string {
  if (typeof window === "undefined") {
    return "http://localhost:9505";
  }
  return window.location.origin;
}

export function joinURL(baseURL: string, path: string): string {
  if (/^https?:\/\//.test(path)) {
    return path;
  }
  return `${baseURL}/${path.replace(/^\/+/, "")}`;
}

export function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}
