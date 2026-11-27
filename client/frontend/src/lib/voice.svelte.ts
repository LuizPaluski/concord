import { AudioPresets, RemoteAudioTrack, Room, RoomEvent, Track } from 'livekit-client'
import { api } from './api.svelte'
import { desktop } from './desktop'
import { noiseFilter } from './noise'
import { callSound, muteSound } from './notify'
import { DEFAULT_LISTEN, RESOLUTIONS, settings } from './settings.svelte'

// Bits por pixel por quadro estimado a partir de presets comuns (1080p60 dá ~8,7 Mbps).
// Se a imagem borrar em jogo rápido, é este número que se ajusta.
const BITS_PER_PIXEL = 0.07
const MAX_BITRATE = 15_000_000

export type Screen = { id: string; name: string; track: Track }
type Grant = { url: string; token: string; at: number }

// O token vale 6 h no servidor; reaproveitar por alguns minutos poupa uma ida à VPS no clique.
const GRANT_TTL = 5 * 60 * 1000

// Call em que eu estava, para voltar a ela quando o app abrir de novo.
const LAST_CALL = 'concord.call'
export type LastCall = { server: number; channel: number; label: string }
export const lastCall = (): LastCall | null => JSON.parse(localStorage.getItem(LAST_CALL) ?? 'null')

class Voice {
  channel = $state(0)
  // Canal em que estou entrando: já aparece na lista dos outros antes de conectar.
  target = $state(0)
  #grants = new Map<number, Grant>()
  #warmed = false
  connecting = $state(false)
  muted = $state(false)
  deafened = $state(false)
  sharing = $state(false)
  picking = $state(false)
  notice = $state('')
  #source = ''
  #mutedBeforeDeafen = false
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

  // Busca o token antes do clique e, na primeira vez, já abre DNS e TLS com o servidor de voz.
  async prefetch(channel: number) {
    const cached = this.#grants.get(channel)
    if (cached && Date.now() - cached.at < GRANT_TTL) return cached
    const grant = { ...(await api<{ url: string; token: string }>(`/api/channels/${channel}/voice`)), at: Date.now() }
    this.#grants.set(channel, grant)
    if (!this.#warmed) {
      this.#warmed = true
      new Room().prepareConnection(grant.url, grant.token).catch(() => {})
    }
    return grant
  }

  async join(channel: number, where?: LastCall) {
    if (this.channel === channel || this.connecting) return
    await this.leave()
    this.connecting = true
    this.target = channel
    const room = new Room({ adaptiveStream: true, dynacast: true })
    this.#room = room
    room
      .on(RoomEvent.TrackSubscribed, (track, _pub, participant) => {
        if (this.#room !== room) return
        if (track instanceof RemoteAudioTrack) {
          const el = track.attach()
          el.muted = this.deafened
          this.#audio.append(el)
          this.#applyListen(participant.identity, track)
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
      const { url, token } = await this.prefetch(channel)
      await room.connect(url, token)
      this.channel = channel
      if (where) localStorage.setItem(LAST_CALL, JSON.stringify(where))
      callSound('join')
      // O microfone (e o filtro de ruído) liga em paralelo: ouvir os outros não espera por ele.
      this.#applyMic().catch((err) => (this.notice = `Microfone não ligou: ${(err as Error).message}`))
    } catch (err) {
      this.#grants.delete(channel)
      await room.disconnect()
      if (this.#room === room) this.#reset()
      throw err
    } finally {
      this.connecting = false
      this.target = 0
    }
  }

  // Sair pela mão esquece a call; fechar o app não passa por aqui.
  async leave() {
    localStorage.removeItem(LAST_CALL)
    await this.#room?.disconnect()
  }

  async toggleMute() {
    this.muted = !this.muted
    if (!this.muted && this.deafened) this.#setDeafened(false)
    muteSound(this.muted)
    await this.#applyMic()
  }

  // Desativar o áudio muta o microfone junto; reativar volta o mic como estava antes.
  async toggleDeafen() {
    const on = !this.deafened
    if (on) this.#mutedBeforeDeafen = this.muted
    this.#setDeafened(on)
    this.muted = on || this.#mutedBeforeDeafen
    muteSound(on)
    await this.#applyMic()
  }

  // Reaplica volume e silêncio que eu defini para essa pessoa.
  applyListen(name: string) {
    const p = this.#room?.remoteParticipants.get(name)
    p?.audioTrackPublications.forEach((pub) => pub.track && this.#applyListen(name, pub.track as RemoteAudioTrack))
  }

  #applyListen(name: string, track: RemoteAudioTrack) {
    const l = { ...DEFAULT_LISTEN, ...settings.listen[name] }
    const screen = track.source === Track.Source.ScreenShareAudio
    const silent = screen ? l.screenMuted : l.muted
    track.setVolume(silent ? 0 : (screen ? l.screenVolume : l.volume) / 100)
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
