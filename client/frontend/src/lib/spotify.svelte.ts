import { auth } from './api.svelte'
import { desktop } from './desktop'

// "Ouvir junto": o Spotify não deixa capturar o áudio, então cada um toca no próprio
// Spotify (Premium) e o Concord só repassa o que o anfitrião está ouvindo.
const CLIENT_ID = import.meta.env.VITE_SPOTIFY_CLIENT_ID as string | undefined
const REDIRECT = 'http://127.0.0.1:43821/callback'
const SCOPES = 'user-read-playback-state user-modify-playback-state user-read-currently-playing'
const KEY = 'concord.spotify'
const POLL = 3000
// Diferença tolerada antes de reposicionar a música de quem ouve junto.
const DRIFT = 3000

type Tokens = { access: string; refresh: string; expires: number }
export type Playing = { uri: string; name: string; artists: string; image: string; position: number; playing: boolean }
export type Jam = { host: string; state: Playing }

const b64url = (bytes: ArrayBuffer) =>
  btoa(String.fromCharCode(...new Uint8Array(bytes)))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '')

class Spotify {
  available = !!desktop && !!CLIENT_ID
  connected = $state(!!localStorage.getItem(KEY))
  jam = $state<Jam | null>(null)
  hosting = $state(false)
  listening = $state(false)
  error = $state('')
  send: (state: Playing | null) => void = () => {}
  #poll: ReturnType<typeof setInterval> | undefined
  #applied: { uri: string; playing: boolean; position: number; at: number } | null = null
  #device = ''

  async login() {
    this.error = ''
    const verifier = b64url(crypto.getRandomValues(new Uint8Array(48)).buffer)
    const challenge = b64url(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier)))
    const state = crypto.randomUUID()
    const url = new URL('https://accounts.spotify.com/authorize')
    url.search = new URLSearchParams({
      client_id: CLIENT_ID!,
      response_type: 'code',
      redirect_uri: REDIRECT,
      code_challenge_method: 'S256',
      code_challenge: challenge,
      scope: SCOPES,
      state,
    }).toString()
    const reply = await desktop!.spotifyLogin(url.toString())
    if (reply.error || reply.state !== state) throw new Error(reply.error === 'access_denied' ? 'login do Spotify cancelado' : 'login do Spotify falhou')
    await this.#exchange({ grant_type: 'authorization_code', code: reply.code, redirect_uri: REDIRECT, client_id: CLIENT_ID!, code_verifier: verifier })
  }

  logout() {
    this.stopHosting()
    this.listening = false
    localStorage.removeItem(KEY)
    this.connected = false
  }

  async #exchange(form: Record<string, string>) {
    const { ok, body } = await desktop!.spotifyToken(form)
    if (!ok) {
      if (form.grant_type === 'refresh_token') this.logout()
      throw new Error(body.error_description || 'o Spotify recusou o login')
    }
    const old: Tokens | null = JSON.parse(localStorage.getItem(KEY) ?? 'null')
    const tokens: Tokens = { access: body.access_token, refresh: body.refresh_token ?? old?.refresh, expires: Date.now() + (body.expires_in - 60) * 1000 }
    localStorage.setItem(KEY, JSON.stringify(tokens))
    this.connected = true
    return tokens
  }

  async #token() {
    let t: Tokens = JSON.parse(localStorage.getItem(KEY) ?? 'null')
    if (!t) throw new Error('conecte o Spotify primeiro')
    if (Date.now() > t.expires) t = await this.#exchange({ grant_type: 'refresh_token', refresh_token: t.refresh, client_id: CLIENT_ID! })
    return t.access
  }

  async #api(path: string, init: { method?: string; body?: unknown } = {}) {
    const res = await fetch('https://api.spotify.com/v1' + path, {
      method: init.method ?? 'GET',
      headers: { Authorization: 'Bearer ' + (await this.#token()), 'Content-Type': 'application/json' },
      body: init.body === undefined ? undefined : JSON.stringify(init.body),
    })
    if (res.status === 204) return null
    if (res.status === 403) throw new Error('precisa de Spotify Premium e estar na lista de usuários do app')
    if (!res.ok) {
      const err = new Error((await res.json().catch(() => null))?.error?.message || `Spotify respondeu ${res.status}`)
      ;(err as Error & { status: number }).status = res.status
      throw err
    }
    return res.json().catch(() => null)
  }

  // Sem Spotify ativo a API responde 404; nesse caso escolhe o Spotify do PC.
  async #play(path: string, body?: unknown) {
    const withDevice = (id: string) => (id ? `${path}${path.includes('?') ? '&' : '?'}device_id=${id}` : path)
    try {
      await this.#api(withDevice(this.#device), { method: 'PUT', body })
    } catch (err) {
      if ((err as { status?: number }).status !== 404) throw err
      const { devices } = await this.#api('/me/player/devices')
      const device = devices.find((d: { type: string }) => d.type === 'Computer') ?? devices[0]
      if (!device) throw new Error('abra o app do Spotify no PC para ouvir junto')
      this.#device = device.id
      await this.#api(withDevice(device.id), { method: 'PUT', body })
    }
  }

  startHosting() {
    this.error = ''
    this.listening = false
    this.hosting = true
    const tick = () => this.#share().catch((err) => (this.error = (err as Error).message))
    tick()
    this.#poll = setInterval(tick, POLL)
  }

  stopHosting() {
    clearInterval(this.#poll)
    if (this.hosting) this.send(null)
    this.hosting = false
  }

  // O anfitrião manda o estado a cada leitura: quem entrar depois pega a posição quase atual.
  async #share() {
    const p = await this.#api('/me/player')
    if (!p?.item || p.currently_playing_type !== 'track') return
    this.send({
      uri: p.item.uri,
      name: p.item.name,
      artists: p.item.artists.map((a: { name: string }) => a.name).join(', '),
      image: p.item.album.images.at(-2)?.url ?? p.item.album.images[0]?.url ?? '',
      position: p.progress_ms,
      playing: p.is_playing,
    })
  }

  onJam(jam: Jam | null) {
    this.jam = jam
    if (!jam) {
      this.listening = false
      this.#applied = null
      return
    }
    if (jam.host === auth.session?.user.name) return
    // Outra pessoa já compartilhava: o servidor ignora a minha, então paro de ler à toa.
    if (this.hosting) this.stopHosting()
    if (this.listening) this.#sync(jam.state).catch((err) => (this.error = (err as Error).message))
  }

  async listen() {
    this.error = ''
    this.listening = true
    this.#applied = null
    if (this.jam) await this.#sync(this.jam.state)
  }

  async stopListening() {
    this.listening = false
    this.#applied = null
    await this.#api('/me/player/pause', { method: 'PUT' }).catch(() => {})
  }

  async #sync(s: Playing) {
    const last = this.#applied
    const now = Date.now()
    if (!last || last.uri !== s.uri) await this.#play('/me/player/play', { uris: [s.uri], position_ms: s.position })
    else if (last.playing !== s.playing) await this.#play(s.playing ? '/me/player/play' : '/me/player/pause', s.playing ? { position_ms: s.position } : undefined)
    else if (s.playing && Math.abs(last.position + (now - last.at) - s.position) > DRIFT) await this.#play(`/me/player/seek?position_ms=${s.position}`)
    else return
    if (!s.playing && (!last || last.uri !== s.uri)) await this.#play('/me/player/pause')
    this.#applied = { uri: s.uri, playing: s.playing, position: s.position, at: now }
  }

  // Saiu da call: para de compartilhar e de seguir quem compartilhava.
  leaveCall() {
    this.stopHosting()
    this.listening = false
    this.#applied = null
    this.jam = null
  }
}

export const spotify = new Spotify()
