export function normalizeColorToHex(input?: string): string {
  if (!input) return '#ffffff';
  const color = input.trim().toLowerCase();
  if (/^#([0-9a-f]{3}|[0-9a-f]{6})$/.test(color)) {
    if (color.length === 4) {
      const r = color[1];
      const g = color[2];
      const b = color[3];
      return `#${r}${r}${g}${g}${b}${b}`;
    }
    return color;
  }

  const rgbMatch = color.match(/^rgba?\(([^)]+)\)$/);
  if (!rgbMatch) return '#ffffff';
  const channels = rgbMatch[1]
    .replace(/\//g, ' ')
    .split(/[,\s]+/)
    .filter(Boolean)
    .slice(0, 3);
  if (channels.length < 3) return '#ffffff';

  const toHex = (value: string) =>
    Math.max(0, Math.min(255, parseInt(value, 10)))
      .toString(16)
      .padStart(2, '0');

  return `#${toHex(channels[0])}${toHex(channels[1])}${toHex(channels[2])}`;
}

export function colorFromCssPaint(input?: string): string | undefined {
  if (!input || input === 'none') return undefined;
  const matches = input.match(/#[0-9a-f]{3,6}\b|rgba?\([^)]+\)/gi);
  return matches ? matches[matches.length - 1] : undefined;
}

export function parseNumberish(input: string | number): number | null {
  const value =
    typeof input === 'number' ? input : parseFloat(String(input).trim());
  return Number.isFinite(value) ? value : null;
}
