import { settings } from './settings.svelte'

let ctx: AudioContext | undefined

export function ping() {
  if (!settings.sound) return
  ctx ??= new AudioContext()
  ctx.resume()
  const t = ctx.currentTime
  const osc = ctx.createOscillator()
  const gain = ctx.createGain()
  osc.frequency.setValueAtTime(880, t)
  osc.frequency.exponentialRampToValueAtTime(1320, t + 0.08)
  gain.gain.setValueAtTime(0.0001, t)
  gain.gain.exponentialRampToValueAtTime(0.12, t + 0.01)
  gain.gain.exponentialRampToValueAtTime(0.0001, t + 0.25)
  osc.connect(gain).connect(ctx.destination)
  osc.start(t)
  osc.stop(t + 0.3)
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
