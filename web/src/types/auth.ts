export interface User {
  id: string
  username: string
  role: string
  isActive: boolean
  authSource?: string   // 'local' | 'sso'
  ssoProvider?: string
  permissions?: string[]
  createdAt?: string
  updatedAt?: string
}

export interface Role {
  id: string
  name: string
  description: string
  permissions: string[]
  builtIn: boolean
  createdAt?: string
  updatedAt?: string
}

export interface PermissionItem {
  key: string
  description: string
}

export interface LoginRequest {
  username: string
  password: string
}

export interface TokenResponse {
  accessToken: string
  refreshToken: string
}

export interface ChangePasswordRequest {
  oldPassword: string
  newPassword: string
}
