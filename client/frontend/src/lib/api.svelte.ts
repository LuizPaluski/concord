export type User = { id: number; name: string }
export type Channel = { id: number; name: string; kind: 'text' | 'voice' }
export type Message = { id: number; channel: number; author: string; body: string; createdAt: number }
export type Presence = { name: string; voice: number }
type Session = { server: string; token: string; user: User }

const saved = localStorage.getItem('concord.session')
export const auth = $state<{ session: Session | null }>({ session: saved ? JSON.parse(saved) : null })

export async function api<T>(path: string, init: { method?: string; body?: unknown } = {}): Promise<T> {
  const s = auth.session!
  const res = await fetch(s.server + path, {
    method: init.method ?? 'GET',
    headers: { Authorization: 'Bearer ' + s.token },
    body: init.body === undefined ? undefined : JSON.stringify(init.body),
  })
  if (res.status === 401) logout()
  if (!res.ok) throw new Error((await res.text()).trim())
  return res.json()
}

export async function enter(server: string, mode: 'login' | 'register', creds: { name: string; password: string; invite?: string }) {
  server = server.trim().replace(/\/+$/, '')
  if (!/^https?:\/\//.test(server)) server = 'https://' + server
  const res = await fetch(server + '/api/' + mode, { method: 'POST', body: JSON.stringify(creds) })
  if (!res.ok) throw new Error((await res.text()).trim())
  const s: Session = { server, ...(await res.json()) }
  localStorage.setItem('concord.server', server)
  localStorage.setItem('concord.session', JSON.stringify(s))
  auth.session = s
}

export function logout() {
  localStorage.removeItem('concord.session')
  auth.session = null
}

export function hue(name: string) {
  let h = 0
  for (const ch of name.toLowerCase()) h = (h * 31 + ch.charCodeAt(0)) % 360
  return h
}
