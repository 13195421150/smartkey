import { expect, it } from 'vitest'
import { formatBeijingTime } from '../beijing-time'

it.each([
  ['2026-09-20T09:50:21.216Z', '2026-09-20 17:50:21'],
  ['2026-09-20T17:50:21+08:00', '2026-09-20 17:50:21'],
  ['2026-09-20 09:50:21', '2026-09-20 17:50:21'],
  ['2026-09-20T20:01:02Z', '2026-09-21 04:01:02'],
  [1784397600, '2026-07-19 02:00:00'],
  [1784397600000, '2026-07-19 02:00:00'],
  [null, '—'], [0, '—'], ['invalid-time', '—'],
])('renders %s in fixed Beijing time', (input, output) => {
  expect(formatBeijingTime(input)).toBe(output)
})
