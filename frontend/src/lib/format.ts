export type Formatted = { value: string; unit: string }

export function formatSpeed(bps: number): Formatted {
  if (bps >= 1_000_000) return { value: (bps / 1_000_000).toFixed(1), unit: 'MB/s' }
  if (bps >= 1_000)     return { value: (bps / 1_000).toFixed(0),     unit: 'KB/s' }
  return                       { value: bps.toFixed(0),                unit:  'B/s' }
}
