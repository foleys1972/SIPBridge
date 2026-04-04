import type { BridgeCallInfo, MIAttendanceRow } from '../api/types'

/** Config user id (bank employee id) from a live call or MI row. */
export function employeeIdFromCall(c: Pick<BridgeCallInfo, 'employee_id' | 'user_id'>): string {
  return (c.employee_id || c.user_id || '').trim()
}

export function employeeIdFromAttendance(r: Pick<MIAttendanceRow, 'employee_id' | 'user_id'>): string {
  return (r.employee_id || r.user_id || '').trim()
}
