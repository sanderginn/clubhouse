export const formatHighlightTimestamp = (seconds: number): string => {
  const safeSeconds = Math.max(0, Math.floor(seconds));
  const hours = Math.floor(safeSeconds / 3600);
  const minutes = Math.floor((safeSeconds % 3600) / 60);
  const remainder = safeSeconds % 60;
  if (hours > 0) {
    return `${hours}:${minutes.toString().padStart(2, '0')}:${remainder.toString().padStart(2, '0')}`;
  }
  return `${minutes.toString().padStart(2, '0')}:${remainder.toString().padStart(2, '0')}`;
};

export const parseHighlightTimestamp = (value: string): number | null => {
  const trimmed = value.trim();
  if (!trimmed) return null;

  const hourMatch = trimmed.match(/^(\d+):([0-5]\d):([0-5]\d)$/);
  if (hourMatch) {
    const hours = Number(hourMatch[1]);
    const minutes = Number(hourMatch[2]);
    const seconds = Number(hourMatch[3]);
    return hours * 3600 + minutes * 60 + seconds;
  }

  const minuteMatch = trimmed.match(/^(\d+):([0-5]\d)$/);
  if (minuteMatch) {
    const minutes = Number(minuteMatch[1]);
    const seconds = Number(minuteMatch[2]);
    return minutes * 60 + seconds;
  }

  return null;
};
