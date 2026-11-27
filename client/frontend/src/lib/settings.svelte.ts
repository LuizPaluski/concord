const KEY = 'concord.settings'

type Settings = { noise: boolean; notify: boolean; sound: boolean }

export const settings = $state<Settings>({ noise: true, notify: true, sound: true, ...JSON.parse(localStorage.getItem(KEY) ?? '{}') })

$effect.root(() => {
  $effect(() => localStorage.setItem(KEY, JSON.stringify(settings)))
})
