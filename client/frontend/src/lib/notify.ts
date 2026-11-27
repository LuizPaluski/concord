import { settings } from './settings.svelte'

let ctx: AudioContext | undefined

// Toca notas curtas: [frequência em Hz, início em s, frequência final opcional para deslizar].
function tone(notes: [number, number, number?][], volume = 0.12) {
  ctx ??= new AudioContext()
  ctx.resume()
  const t = ctx.currentTime
  for (const [freq, start, glide] of notes) {
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()
    osc.frequency.setValueAtTime(freq, t + start)
    if (glide) osc.frequency.exponentialRampToValueAtTime(glide, t + start + 0.08)
    gain.gain.setValueAtTime(0.0001, t + start)
    gain.gain.exponentialRampToValueAtTime(volume, t + start + 0.01)
    gain.gain.exponentialRampToValueAtTime(0.0001, t + start + 0.18)
    osc.connect(gain).connect(ctx.destination)
    osc.start(t + start)
    osc.stop(t + start + 0.2)
  }
}

export function ping() {
  if (settings.sound) tone([[880, 0, 1320]])
}

export function callSound(kind: 'join' | 'leave') {
  if (settings.callSound) tone(kind === 'join' ? [[523, 0], [784, 0.09]] : [[784, 0], [523, 0.09]], 0.1)
}

export function muteSound(on: boolean) {
  if (settings.callSound) tone(on ? [[660, 0, 440]] : [[440, 0, 660]], 0.09)
}

// Tiro sintetizado: estalo de ruído com decaimento rápido e um baque grave por baixo.
export function gunshot() {
  ctx ??= new AudioContext()
  ctx.resume()
  const t = ctx.currentTime
  const noise = ctx.createBuffer(1, ctx.sampleRate * 0.6, ctx.sampleRate)
  const data = noise.getChannelData(0)
  for (let i = 0; i < data.length; i++) data[i] = (Math.random() * 2 - 1) * Math.exp(-i / (ctx.sampleRate * 0.07))
  const src = ctx.createBufferSource()
  src.buffer = noise
  const crack = ctx.createGain()
  crack.gain.value = 0.9
  src.connect(crack).connect(ctx.destination)
  src.start(t)

  const boom = ctx.createOscillator()
  const gain = ctx.createGain()
  boom.frequency.setValueAtTime(120, t)
  boom.frequency.exponentialRampToValueAtTime(40, t + 0.25)
  gain.gain.setValueAtTime(0.8, t)
  gain.gain.exponentialRampToValueAtTime(0.0001, t + 0.3)
  boom.connect(gain).connect(ctx.destination)
  boom.start(t)
  boom.stop(t + 0.3)
}

export function showNotification(title: string, body: string, onclick: () => void) {
  if (!settings.notify || !('Notification' in window) || Notification.permission !== 'granted') return
  const n = new Notification(title, { body, silent: true })
  n.onclick = () => {
    window.focus()
    onclick()
  }
}

export function askPermission() {
  if ('Notification' in window && Notification.permission === 'default') Notification.requestPermission()
}
