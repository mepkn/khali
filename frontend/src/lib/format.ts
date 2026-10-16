/** Decimal units, like Finder: 1 GB = 1,000,000,000 bytes. */
export function formatBytes(bytes: number): string {
  if (bytes < 1000) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let v = bytes;
  let i = -1;
  while (v >= 1000 && i < units.length - 1) {
    v /= 1000;
    i++;
  }
  return `${v >= 100 ? v.toFixed(0) : v.toFixed(1)} ${units[i]}`;
}

/** "today", "yesterday", "5 days ago", "3 months ago". */
export function formatAgo(unixSeconds: number, now = Date.now()): string {
  if (!unixSeconds) return "never";
  const days = Math.floor((now / 1000 - unixSeconds) / 86400);
  if (days <= 0) return "today";
  if (days === 1) return "yesterday";
  if (days < 30) return `${days} days ago`;
  const months = Math.floor(days / 30);
  if (months < 12) return months === 1 ? "a month ago" : `${months} months ago`;
  const years = Math.floor(days / 365);
  return years <= 1 ? "a year ago" : `${years} years ago`;
}

export function plural(n: number, word: string, many = word + "s"): string {
  return `${n} ${n === 1 ? word : many}`;
}

/** The folder a regenerate command runs in: the parent of a target's rel path. */
export function parentRel(rel: string): string {
  const i = rel.lastIndexOf("/");
  return i < 0 ? "." : rel.slice(0, i);
}
