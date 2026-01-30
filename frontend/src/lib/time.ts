export const normalizeTimezone = (timezone?: string | null): string | undefined => {
  if (!timezone) return undefined;
  const trimmed = timezone.trim();
  return trimmed.length > 0 ? trimmed : undefined;
};

export const formatInTimezone = (
  date: Date,
  options: Intl.DateTimeFormatOptions,
  timezone?: string | null
): string => {
  const normalized = normalizeTimezone(timezone);
  const formatter = new Intl.DateTimeFormat('en-US', {
    ...options,
    ...(normalized ? { timeZone: normalized } : {}),
  });
  return formatter.format(date);
};

export const getYearInTimezone = (date: Date, timezone?: string | null): number => {
  const normalized = normalizeTimezone(timezone);
  if (!normalized) return date.getFullYear();
  const parts = new Intl.DateTimeFormat('en-US', {
    year: 'numeric',
    timeZone: normalized,
  }).formatToParts(date);
  const yearPart = parts.find((part) => part.type === 'year');
  const year = yearPart ? Number(yearPart.value) : NaN;
  return Number.isNaN(year) ? date.getFullYear() : year;
};
