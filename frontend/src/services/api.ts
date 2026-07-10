const BASE_URL = '/api'

function getToken(): string {
  return localStorage.getItem('shelly_token') || ''
}

async function request<T = any>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string> || {}),
  }
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const res = await fetch(`${BASE_URL}${path}`, { ...options, headers })
  if (res.status === 401) {
    localStorage.removeItem('shelly_token')
    localStorage.removeItem('shelly_user')
    window.location.reload()
    throw new Error('Unauthorized')
  }
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || 'Request failed')
  return data
}

// Auth
export const authApi = {
  login: (username: string, password: string) =>
    request('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) }),
  register: (username: string, password: string) =>
    request('/auth/register', { method: 'POST', body: JSON.stringify({ username, password }) }),
  getProfile: () => request('/profile'),
  changePassword: (oldPassword: string, newPassword: string) =>
    request('/profile/password', { method: 'PUT', body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }) }),
}

// Assets
export const assetApi = {
  list: (params?: { group_id?: string; search?: string; type?: string }) => {
    const qs = new URLSearchParams(params as Record<string, string>).toString()
    return request(`/assets${qs ? '?' + qs : ''}`)
  },
  get: (id: number) => request(`/assets/${id}`),
  create: (data: any) => request('/assets', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: number, data: any) => request(`/assets/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: number) => request(`/assets/${id}`, { method: 'DELETE' }),
  batchDelete: (ids: number[]) => request('/assets/batch-delete', { method: 'POST', body: JSON.stringify({ ids }) }),
}

// Groups
export const groupApi = {
  list: () => request('/groups'),
  create: (data: any) => request('/groups', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: number, data: any) => request(`/groups/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: number) => request(`/groups/${id}`, { method: 'DELETE' }),
}

// Snippets
export const snippetApi = {
  list: () => request('/snippets'),
  create: (data: any) => request('/snippets', { method: 'POST', body: JSON.stringify(data) }),
  delete: (id: number) => request(`/snippets/${id}`, { method: 'DELETE' }),
}

// Highlights
export const highlightApi = {
  list: () => request('/highlights'),
  create: (data: any) => request('/highlights', { method: 'POST', body: JSON.stringify(data) }),
  delete: (id: number) => request(`/highlights/${id}`, { method: 'DELETE' }),
}

// SFTP
export const sftpApi = {
  list: (assetId: number, path: string) => request(`/sftp/${assetId}/list?path=${encodeURIComponent(path)}`),
  upload: (assetId: number, file: File, remotePath: string) => {
    const formData = new FormData()
    formData.append('file', file)
    formData.append('path', remotePath)
    return fetch(`${BASE_URL}/sftp/${assetId}/upload`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${getToken()}` },
      body: formData,
    }).then(r => r.json())
  },
  download: (assetId: number, path: string) => `${BASE_URL}/sftp/${assetId}/download?path=${encodeURIComponent(path)}&token=${getToken()}`,
  batchDownload: (assetId: number, paths: string[]) =>
    request(`/sftp/${assetId}/batch-download`, { method: 'POST', body: JSON.stringify({ paths }) }),
  mkdir: (assetId: number, path: string) =>
    request(`/sftp/${assetId}/mkdir`, { method: 'POST', body: JSON.stringify({ path }) }),
  rename: (assetId: number, oldPath: string, newPath: string) =>
    request(`/sftp/${assetId}/rename`, { method: 'POST', body: JSON.stringify({ old_path: oldPath, new_path: newPath }) }),
  delete: (assetId: number, paths: string[]) =>
    request(`/sftp/${assetId}/delete`, { method: 'POST', body: JSON.stringify({ paths }) }),
}

// Batch Exec
export const batchExecApi = {
  exec: (assetIds: number[], command: string) =>
    request('/batch/exec', { method: 'POST', body: JSON.stringify({ asset_ids: assetIds, command }) }),
}

// Port Forward
export const portForwardApi = {
  list: () => request('/port-forward/rules'),
  create: (data: any) => request('/port-forward/rules', { method: 'POST', body: JSON.stringify(data) }),
  delete: (id: number) => request(`/port-forward/rules/${id}`, { method: 'DELETE' }),
  start: (id: number) => request(`/port-forward/rules/${id}/start`, { method: 'POST' }),
  stop: (id: number) => request(`/port-forward/rules/${id}/stop`, { method: 'POST' }),
  status: () => request('/port-forward/status'),
}

// Sessions
export const sessionApi = {
  list: () => request('/sessions'),
  getRecord: (id: number) => `${BASE_URL}/sessions/${id}/record?token=${getToken()}`,
  download: (id: number) => `${BASE_URL}/sessions/${id}/download?token=${getToken()}`,
  delete: (id: number) => request(`/sessions/${id}`, { method: 'DELETE' }),
}

// AI
export const aiApi = {
  chat: (sessionId: number | undefined, message: string, context: string, model?: string) =>
    fetch(`${BASE_URL}/ai/chat`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
      body: JSON.stringify({ session_id: sessionId, message, context, model }),
    }),
  listSessions: () => request('/ai/sessions'),
  getHistory: (sessionId: number) => request(`/ai/sessions/${sessionId}/history`),
  deleteSession: (sessionId: number) => request(`/ai/sessions/${sessionId}`, { method: 'DELETE' }),
}

// Sync
export const syncApi = {
  getConfig: () => request('/sync/config'),
  updateConfig: (data: any) => request('/sync/config', { method: 'PUT', body: JSON.stringify(data) }),
  trigger: () => request('/sync/trigger', { method: 'POST' }),
}

// Settings
export const settingsApi = {
  get: () => request('/settings'),
  update: (data: any) => request('/settings', { method: 'PUT', body: JSON.stringify(data) }),
}

// API Tokens
export const tokenApi = {
  list: () => request('/tokens'),
  create: (name: string) => request('/tokens', { method: 'POST', body: JSON.stringify({ name }) }),
  delete: (id: number) => request(`/tokens/${id}`, { method: 'DELETE' }),
}
