const KEY = 'concord.settings'

export const RESOLUTIONS = { 720: [1280, 720], 1080: [1920, 1080], 1440: [2560, 1440] } as const
export const FRAME_RATES = [15, 30, 60] as const
export const THEMES = {
  escuro: 'Escuro',
  'meia-noite': 'Meia-noite',
  claro: 'Claro',
  ameixa: 'Ameixa',
  oceano: 'Oceano',
  floresta: 'Floresta',
  brasa: 'Brasa',
} as const

type Settings = {
  noise: boolean
  notify: boolean
  sound: boolean
  callSound: boolean
  screenRes: keyof typeof RESOLUTIONS
  screenFps: (typeof FRAME_RATES)[number]
  screenAudio: boolean
  theme: keyof typeof THEMES
  // Ajustes que cada um faz só para si sobre o áudio dos outros, por nome.
  listen: Record<string, Listen>
  hideTiles: boolean
}

export type Listen = { volume: number; muted: boolean; screenVolume: number; screenMuted: boolean }
export const DEFAULT_LISTEN: Listen = { volume: 100, muted: false, screenVolume: 100, screenMuted: false }

export const settings = $state<Settings>({
  noise: true,
  notify: true,
  sound: true,
  callSound: true,
  screenRes: 1080,
  screenFps: 60,
  screenAudio: true,
  theme: 'escuro',
  listen: {},
  hideTiles: false,
  ...JSON.parse(localStorage.getItem(KEY) ?? '{}'),
})

$effect.root(() => {
  $effect(() => localStorage.setItem(KEY, JSON.stringify(settings)))
  $effect(() => {
    document.documentElement.dataset.theme = settings.theme
  })
})
