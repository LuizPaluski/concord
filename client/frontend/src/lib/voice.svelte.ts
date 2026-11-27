import { Room, RoomEvent, Track } from 'livekit-client'
import { api } from './api.svelte'
import { noiseFilter } from './noise'
import { callSound } from './notify'
import { RESOLUTIONS, settings } from './settings.svelte'

// Bits por pixel por quadro estimado a partir de presets comuns (1080p60 dá ~8,7 Mbps).
// Se a imagem borrar em jogo rápido, é este número que se ajusta.
const BITS_PER_PIXEL = 0.07
const MAX_BITRATE = 15_000_000

export type Screen = { id: string; name: string; track: Track }

class Voice {
  channel = $state(0)
  connecting = $state(false)
  muted = $state(false)
  deafened = $state(false)
  sharing = $state(false)
  noiseError = $state('')
  speaking = $state<string[]>([])
  screens = $state.raw<Screen[]>([])
  #room: Room | null = null
  #audio = document.createElement('div')

  constructor() {
    this.#audio.hidden = true
    document.body.append(this.#audio)
  }

  async join(channel: number) {
    if (this.channel === channel || this.connecting) return
    await this.leave()
    this.connecting = true
    const room = new Room({ adaptiveStream: true, dynacast: true })
    this.#room = room
    room
      .on(RoomEvent.TrackSubscribed, (track) => {
        if (this.#room !== room) return
        if (track.kind === Track.Kind.Audio) {
          const el = track.attach()
          el.muted = this.deafened
          this.#audio.append(el)
        }
        this.#refresh()
      })
      .on(RoomEvent.TrackUnsubscribed, (track) => {
        track.detach().forEach((el) => el.remove())
        this.#refresh()
      })
      .on(RoomEvent.ParticipantConnected, () => {
        if (this.#room === room) callSound('join')
      })
      .on(RoomEvent.ParticipantDisconnected, () => {
        if (this.#room === room) callSound('leave')
      })
      .on(RoomEvent.LocalTrackPublished, () => this.#refresh())
      .on(RoomEvent.LocalTrackUnpublished, () => this.#refresh())
      .on(RoomEvent.ActiveSpeakersChanged, (ps) => {
        if (this.#room === room) this.speaking = ps.map((p) => p.identity)
      })
      // Sem essa checagem, o aviso atrasado de uma sala antiga zerava a sala nova:
      // o áudio seguia tocando, mas as telas compartilhadas nunca apareciam.
      .on(RoomEvent.Disconnected, () => {
        if (this.#room !== room) return
        if (this.channel) callSound('leave')
        this.#reset()
      })
    try {
      const { url, token } = await api<{ url: string; token: string }>(`/api/channels/${channel}/voice`)
      await room.connect(url, token)
      this.channel = channel
      callSound('join')
      await this.#applyMic()
    } catch (err) {
      await room.disconnect()
      if (this.#room === room) this.#reset()
      throw err
    } finally {
      this.connecting = false
    }
  }

  async leave() {
    await this.#room?.disconnect()
  }

  async toggleMute() {
    this.muted = !this.muted
    if (!this.muted && this.deafened) this.#setDeafened(false)
    await this.#applyMic()
  }

  async toggleDeafen() {
    this.#setDeafened(!this.deafened)
    await this.#applyMic()
  }

  async setNoise(on: boolean) {
    settings.noise = on
    await this.#applyNoise()
  }

  async toggleScreen() {
    const lp = this.#room?.localParticipant
    if (!lp) return
    const [width, height] = RESOLUTIONS[settings.screenRes]
    const fps = settings.screenFps
    // A 60 fps é jogo ou vídeo: vale perder nitidez para não engasgar. Abaixo disso,
    // costuma ser código ou documento, e a nitidez do texto importa mais.
    const motion = fps >= 60
    const maxBitrate = Math.min(MAX_BITRATE, Math.round(width * height * fps * BITS_PER_PIXEL))
    try {
      await lp.setScreenShareEnabled(
        !this.sharing,
        {
          audio: settings.screenAudio,
          systemAudio: 'include',
          resolution: { width, height, frameRate: fps },
          contentHint: motion ? 'motion' : 'detail',
        },
        {
          // H.264 costuma ter codificador em hardware; VP8 a 1080p60 pesa na CPU.
          videoCodec: 'h264',
          // Uma camada só, na qualidade escolhida. Com simulcast o LiveKit manda uma
          // versão reduzida e segura a cheia até achar que quem assiste aguenta.
          simulcast: false,
          screenShareEncoding: { maxBitrate, maxFramerate: fps },
          degradationPreference: motion ? 'maintain-framerate' : 'maintain-resolution',
        },
      )
    } catch (err) {
      // Fechar o seletor de tela sem escolher nada não é erro.
      if ((err as Error).name !== 'NotAllowedError') throw err
    }
    this.#refresh()
  }

  #setDeafened(on: boolean) {
    this.deafened = on
    this.#audio.querySelectorAll('audio').forEach((el) => (el.muted = on))
  }

  async #applyMic() {
    await this.#room?.localParticipant.setMicrophoneEnabled(!this.muted && !this.deafened)
    await this.#applyNoise()
  }

  // A trilha do microfone só existe depois da primeira vez que ele é ligado.
  // Se o filtro falhar, a chamada segue sem ele: perder a voz é pior que ouvir o teclado.
  async #applyNoise() {
    const mic = this.#room?.localParticipant.getTrackPublication(Track.Source.Microphone)?.audioTrack
    if (!mic) return
    try {
      if (settings.noise && !mic.getProcessor()) await mic.setProcessor(noiseFilter())
      else if (!settings.noise && mic.getProcessor()) await mic.stopProcessor()
      this.noiseError = ''
    } catch (err) {
      this.noiseError = (err as Error).message || 'falha ao iniciar o filtro'
    }
  }

  #refresh() {
    const room = this.#room
    if (!room) return
    this.sharing = room.localParticipant.isScreenShareEnabled
    this.screens = [room.localParticipant, ...room.remoteParticipants.values()].flatMap((p) => {
      const track = p.getTrackPublication(Track.Source.ScreenShare)?.track
      return track ? [{ id: p.identity, name: p.name || p.identity, track }] : []
    })
  }

  #reset() {
    this.#room = null
    this.channel = 0
    this.sharing = false
    this.speaking = []
    this.screens = []
    this.#audio.replaceChildren()
  }
}

export const voice = new Voice()
