export function hasPermission(grants: readonly string[], permission: string): boolean {
  return grants.includes("*") || grants.includes(permission);
}
