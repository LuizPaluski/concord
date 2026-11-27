import { loadRnnoise, RnnoiseWorkletNode } from '@sapphi-red/web-noise-suppressor'
import workletUrl from '@sapphi-red/web-noise-suppressor/rnnoiseWorklet.js?url'
import wasmUrl from '@sapphi-red/web-noise-suppressor/rnnoise.wasm?url'
import simdUrl from '@sapphi-red/web-noise-suppressor/rnnoise_simd.wasm?url'
import type { AudioProcessorOptions, Track, TrackProcessor } from 'livekit-client'

let wasm: Promise<ArrayBuffer> | undefined

export function noiseFilter(): TrackProcessor<Track.Kind.Audio, AudioProcessorOptions> {
  let ctx: AudioContext | undefined
  const filter: TrackProcessor<Track.Kind.Audio, AudioProcessorOptions> = {
    name: 'rnnoise',
    async init({ track }) {
      // O RNNoise só trabalha a 48 kHz, por isso um contexto próprio em vez do da sala.
      ctx = new AudioContext({ sampleRate: 48000, latencyHint: 'interactive' })
      wasm ??= loadRnnoise({ url: wasmUrl, simdUrl }).catch((err) => {
        wasm = undefined
        throw err
      })
      try {
        const [binary] = await Promise.all([wasm, ctx.audioWorklet.addModule(workletUrl)])
        const out = ctx.createMediaStreamDestination()
        ctx
          .createMediaStreamSource(new MediaStream([track]))
          .connect(new RnnoiseWorkletNode(ctx, { wasmBinary: binary, maxChannels: 1 }))
          .connect(out)
        await ctx.resume()
        filter.processedTrack = out.stream.getAudioTracks()[0]
      } catch (err) {
        // O LiveKit não chama destroy quando o init falha; sem isso o contexto fica aberto.
        await filter.destroy()
        throw err
      }
    },
    async restart(opts) {
      await filter.destroy()
      await filter.init(opts)
    },
    async destroy() {
      await ctx?.close()
      ctx = undefined
      filter.processedTrack = undefined
    },
  }
  return filter
}
