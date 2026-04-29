const MEDIUM_DATE = new Intl.DateTimeFormat("en-US", { dateStyle: "medium" });

export function formatMediumDate(iso: string): string {
  return MEDIUM_DATE.format(new Date(iso));
}
