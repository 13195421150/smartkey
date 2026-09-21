/** API timestamps are UTC unless an explicit offset is supplied. */
export function formatBeijingTime(value: unknown): string {
  if (value == null || value === '' || value === 0 || value === '0') return '—'
  let date: Date
  if (typeof value === 'number' || /^\d+(?:\.\d+)?$/.test(String(value))) {
    const n = Number(value)
    date = new Date(n < 1e12 ? n * 1000 : n)
  } else {
    let text = String(value).trim().replace(' ', 'T')
    if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?$/.test(text)) text += 'Z'
    date = new Date(text)
  }
  if (!Number.isFinite(date.getTime())) return '—'
  const parts = new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23',
  }).formatToParts(date)
  const p = Object.fromEntries(parts.map(part => [part.type, part.value]))
  return `${p.year}-${p.month}-${p.day} ${p.hour}:${p.minute}:${p.second}`
}
