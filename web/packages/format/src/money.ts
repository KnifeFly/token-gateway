export function formatMoney(value: number, maximumFractionDigits = 4): string {
  return new Intl.NumberFormat("en", { maximumFractionDigits }).format(value);
}
