import { describe, it, expect } from 'vitest'
import { canRotateCDK } from '../cdk-rotation'

describe('卡密换新入口', () => {
  it('只允许有完整码和固定 ID 的未使用 CDK', () => {
    expect(canRotateCDK({ id: 7, status: 'unused', fullCode: 'PLUS-SYNTHETIC-CODE' })).toBe(true)
    for (const status of ['consumed', 'reserved', 'review', 'frozen', 'disabled', '', undefined]) {
      expect(canRotateCDK({ id: 7, status, fullCode: 'PLUS-SYNTHETIC-CODE' })).toBe(false)
    }
    expect(canRotateCDK({ id: 0, status: 'unused', fullCode: 'PLUS-SYNTHETIC-CODE' })).toBe(false)
    expect(canRotateCDK({ id: 7, status: 'unused', fullCode: '' })).toBe(false)
  })
})
