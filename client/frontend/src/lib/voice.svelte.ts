import { AudioPresets, Room, RoomEvent, Track } from 'livekit-client'
import { api } from './api.svelte'
import { desktop } from './desktop'
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
  picking = $state(false)
  notice = $state('')
  #source = ''
  #screenAudio: { ctx: AudioContext; track: MediaStreamTrack } | null = null
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
      .on(RoomEvent.LocalTrackUnpublished, (pub) => {
        // Janela compartilhada fechou: o vídeo sai sozinho, o som nativo não.
        if (pub.source === Track.Source.ScreenShare) this.#stopScreenAudio().catch(() => {})
        this.#refresh()
      })
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
    if (this.sharing) {
      await this.#stopScreenAudio()
      await lp.setScreenShareEnabled(false)
      this.#refresh()
      return
    }
    // No app, o seletor é nosso (Electron); no navegador, fica o do Chromium.
    if (desktop) this.picking = true
    else await this.#startScreen(settings.screenAudio)
  }

  async shareSource(id: string) {
    this.picking = false
    this.#source = id
    // Sem loopback por processo (Windows 10 antigo), o som vai pelo próprio getDisplayMedia.
    const native = await desktop!.nativeScreenAudio()
    await this.#startScreen(settings.screenAudio && !native)
    if (native && this.sharing && settings.screenAudio) {
      await this.#startScreenAudio().catch((err) => {
        this.notice = `Transmitindo sem som: ${(err as Error).message}`
        return this.#stopScreenAudio()
      })
    }
  }

  // Som da tela capturado pelo concord-audio (nativo), publicado como trilha à parte.
  async #startScreenAudio() {
    const ctx = new AudioContext({ sampleRate: 48000 })
    const output = ctx.createMediaStreamDestination()
    const audio = { ctx, track: output.stream.getAudioTracks()[0] }
    this.#screenAudio = audio
    await ctx.audioWorklet.addModule('/pcm-worklet.js')
    const player = new AudioWorkletNode(ctx, 'pcm-player', { outputChannelCount: [2] })
    player.connect(output)
    await desktop!.startScreenAudio(
      this.#source,
      (pcm) => {
        const copy = pcm.slice()
        player.port.postMessage(copy.buffer, [copy.buffer])
      },
      (message) => {
        this.notice = `O som da tela parou: ${message}`
        this.#stopScreenAudio().catch(() => {})
      },
    )
    const lp = this.#room!.localParticipant
    await lp.publishTrack(audio.track, {
      source: Track.Source.ScreenShareAudio,
      name: 'screen_share_audio',
      audioPreset: AudioPresets.musicHighQualityStereo,
      forceStereo: true,
      dtx: false,
      red: false,
    })
    // Se o áudio caiu enquanto publicava, a trilha subiu já morta e precisa sair.
    if (this.#screenAudio !== audio) await lp.unpublishTrack(audio.track)
  }

  async #stopScreenAudio() {
    const audio = this.#screenAudio
    if (!audio) return
    this.#screenAudio = null
    await desktop!.stopScreenAudio()
    await this.#room?.localParticipant.unpublishTrack(audio.track)
    audio.track.stop()
    await audio.ctx.close()
  }

  async #startScreen(audio: boolean) {
    const lp = this.#room?.localParticipant
    if (!lp) return
    // O processo principal consome a fonte a cada pedido, então ela vai de novo em cada tentativa.
    if (desktop) await desktop.chooseScreen(this.#source)
    const [width, height] = RESOLUTIONS[settings.screenRes]
    const fps = settings.screenFps
    // A 60 fps é jogo ou vídeo: vale perder nitidez para não engasgar. Abaixo disso,
    // costuma ser código ou documento, e a nitidez do texto importa mais.
    const motion = fps >= 60
    const maxBitrate = Math.min(MAX_BITRATE, Math.round(width * height * fps * BITS_PER_PIXEL))
    try {
      await lp.setScreenShareEnabled(
        true,
        {
          audio,
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
      const e = err as Error
      // O som do sistema às vezes não abre no Windows ("Could not start audio source").
      // A tela sem som é melhor que nenhuma transmissão.
      if (audio && (e.name === 'NotReadableError' || /audio source/i.test(e.message))) {
        await this.#startScreen(false)
        this.notice = 'Transmitindo sem som: o Windows não liberou o áudio do sistema.'
        return
      }
      // No navegador, NotAllowedError é só o seletor do Chromium fechado sem escolha.
      // No app o seletor é nosso, então esse erro é falha de verdade e precisa aparecer.
      if (desktop || e.name !== 'NotAllowedError') throw err
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
    this.#stopScreenAudio().catch(() => {})
    this.#room = null
    this.channel = 0
    this.sharing = false
    this.picking = false
    this.speaking = []
    this.screens = []
    this.#audio.replaceChildren()
  }
}

export const voice = new Voice()
