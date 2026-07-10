import { create } from 'zustand'

interface AuthState {
  token: string
  user: { id: number; username: string; role: string } | null
  setAuth: (token: string, user: any) => void
  logout: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  token: localStorage.getItem('shelly_token') || '',
  user: JSON.parse(localStorage.getItem('shelly_user') || 'null'),
  setAuth: (token, user) => {
    localStorage.setItem('shelly_token', token)
    localStorage.setItem('shelly_user', JSON.stringify(user))
    set({ token, user })
  },
  logout: () => {
    localStorage.removeItem('shelly_token')
    localStorage.removeItem('shelly_user')
    set({ token: '', user: null })
  },
}))
