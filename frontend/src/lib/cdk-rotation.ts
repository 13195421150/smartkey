export function canRotateCDK(row: { id?: unknown; status?: unknown; fullCode?: unknown }) {
  const id = Number(row?.id)
  return Number.isInteger(id) && id > 0 && row?.status === 'unused' && !!row.fullCode
}
