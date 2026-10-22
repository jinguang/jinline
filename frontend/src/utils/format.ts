export function fmtTime(v?: string | null): string {
  if (!v) return '-'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return '-'
  return d.toLocaleString('zh-CN', { hour12: false })
}

export function fmtDuration(run: { started_at?: string | null; finished_at?: string | null }): string {
  if (!run.started_at) return '-'
  const start = new Date(run.started_at).getTime()
  const end = run.finished_at ? new Date(run.finished_at).getTime() : Date.now()
  const ms = Math.max(0, end - start)
  if (ms < 1000) return `${ms}ms`
  const seconds = Math.floor(ms / 1000)
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  const rest = seconds % 60
  if (minutes < 60) return `${minutes}m${rest.toString().padStart(2, '0')}s`
  const hours = Math.floor(minutes / 60)
  return `${hours}h${(minutes % 60).toString().padStart(2, '0')}m`
}

export function fmtDurationMs(ms?: number): string {
  if (!ms || ms < 0) return '-'
  if (ms < 1000) return `${ms}ms`
  const seconds = Math.floor(ms / 1000)
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  return `${minutes}m${(seconds % 60).toString().padStart(2, '0')}s`
}

export function shortSha(sha?: string): string {
  return sha ? sha.slice(0, 8) : '-'
}
