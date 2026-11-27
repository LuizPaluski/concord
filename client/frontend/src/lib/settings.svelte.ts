const KEY = 'concord.settings'

export const RESOLUTIONS = { 720: [1280, 720], 1080: [1920, 1080], 1440: [2560, 1440] } as const
export const FRAME_RATES = [15, 30, 60] as const

type Settings = {
  noise: boolean
  notify: boolean
  sound: boolean
  callSound: boolean
  screenRes: keyof typeof RESOLUTIONS
  screenFps: (typeof FRAME_RATES)[number]
  screenAudio: boolean
}

export const settings = $state<Settings>({
  noise: true,
  notify: true,
  sound: true,
  callSound: true,
  screenRes: 1080,
  screenFps: 60,
  screenAudio: true,
  ...JSON.parse(localStorage.getItem(KEY) ?? '{}'),
})

$effect.root(() => {
  $effect(() => localStorage.setItem(KEY, JSON.stringify(settings)))
})
