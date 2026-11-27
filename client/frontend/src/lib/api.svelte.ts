export type User = { id: number; name: string }
export type Profile = User & { avatar: string; banner: string; color: string; color2: string; effect: Effect; bio: string }
export type Effect = '' | 'gradient' | 'glow' | 'rainbow'
export type Guild = { id: number; name: string; icon: string; owner: number }
export type Channel = { id: number; server: number; name: string; kind: 'text' | 'voice' }
export type Attachment = { id: string; mime: string; name: string; size: number }
export type Message = { id: number; channel: number; author: string; authorId: number; body: string; file: Attachment | null; createdAt: number }
export type Presence = { name: string; voice: number; muted: boolean; deafened: boolean; sharing: boolean }
type Session = { server: string; token: string; user: User }

const saved = localStorage.getItem('concord.session')
export const auth = $state<{ session: Session | null }>({ session: saved ? JSON.parse(saved) : null })

// Perfis de quem divide grupo comigo, por nome: presença e voz só conhecem o nome.
export const people = $state<Record<string, Profile>>({})

export async function api<T>(path: string, init: { method?: string; body?: unknown } = {}): Promise<T> {
  const s = auth.session!
  const res = await fetch(s.server + path, {
    method: init.method ?? 'GET',
    headers: { Authorization: 'Bearer ' + s.token },
    body: init.body instanceof Blob ? init.body : init.body === undefined ? undefined : JSON.stringify(init.body),
  })
  if (res.status === 401) logout()
  if (!res.ok) throw new Error((await res.text()).trim())
  return res.status === 204 ? (undefined as T) : res.json()
}

export const mediaUrl = (id: string) => (id ? `${auth.session!.server}/media/${id}` : '')
export const fileUrl = (id: string) => `${auth.session!.server}/files/${id}`

export const uploadFile = (file: File) => api<Attachment>(`/api/files?name=${encodeURIComponent(file.name)}`, { method: 'POST', body: file })

export async function loadPeople() {
  for (const p of await api<Profile[]>('/api/users')) people[p.name] = p
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

// Recorta e reduz no próprio cliente para o upload ficar em poucos KB.
// GIF sobe como está para não perder a animação.
export async function uploadImage(file: File, width: number, height: number) {
  if (file.type === 'image/gif') return (await api<{ id: string }>('/api/media', { method: 'POST', body: file })).id
  const img = await createImageBitmap(file)
  const scale = Math.max(width / img.width, height / img.height)
  const canvas = new OffscreenCanvas(width, height)
  canvas.getContext('2d')!.drawImage(img, (width - img.width * scale) / 2, (height - img.height * scale) / 2, img.width * scale, img.height * scale)
  const blob = await canvas.convertToBlob({ type: 'image/webp', quality: 0.9 })
  return (await api<{ id: string }>('/api/media', { method: 'POST', body: blob })).id
}
