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
