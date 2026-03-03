/**
 * 检查用户是否拥有指定权限
 */
export function hasPermission(permissions: string[], perm: string): boolean {
  return permissions.includes(perm)
}

/**
 * 检查用户是否拥有任一权限
 */
export function hasAnyPermission(permissions: string[], perms: string[]): boolean {
  return perms.some((p) => permissions.includes(p))
}
