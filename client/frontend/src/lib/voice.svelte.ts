import { Room, RoomEvent, ScreenSharePresets, Track } from 'livekit-client'
import { api } from './api.svelte'

export type Screen = { id: string; name: string; track: Track }

class Voice {
  channel = $state(0)
  connecting = $state(false)
  muted = $state(false)
  deafened = $state(false)
  sharing = $state(false)
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
      .on(RoomEvent.LocalTrackPublished, () => this.#refresh())
      .on(RoomEvent.LocalTrackUnpublished, () => this.#refresh())
      .on(RoomEvent.ActiveSpeakersChanged, (ps) => (this.speaking = ps.map((p) => p.identity)))
      .on(RoomEvent.Disconnected, () => this.#reset())
    try {
      const { url, token } = await api<{ url: string; token: string }>(`/api/channels/${channel}/voice`)
      await room.connect(url, token)
      this.channel = channel
      await this.#applyMic()
    } catch (err) {
      await room.disconnect()
      this.#reset()
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

  async toggleScreen() {
    const lp = this.#room?.localParticipant
    if (!lp) return
    try {
      await lp.setScreenShareEnabled(
        !this.sharing,
        { audio: true, resolution: ScreenSharePresets.h1080fps30.resolution },
        { screenShareEncoding: ScreenSharePresets.h1080fps30.encoding },
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

  #applyMic() {
    return this.#room?.localParticipant.setMicrophoneEnabled(!this.muted && !this.deafened)
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
