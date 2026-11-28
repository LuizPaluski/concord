const { contextBridge, ipcRenderer } = require('electron')

function forgetScreenAudio() {
  ipcRenderer.removeAllListeners('screen:audio')
  ipcRenderer.removeAllListeners('screen:audio-error')
}

contextBridge.exposeInMainWorld('desktop', {
  minimize: () => ipcRenderer.send('window:minimize'),
  toggleMaximize: () => ipcRenderer.send('window:toggle-maximize'),
  close: () => ipcRenderer.send('window:close'),
  isMaximized: () => ipcRenderer.invoke('window:is-maximized'),
  onMaximized: (fn) => ipcRenderer.on('window:maximized', (_, maximized) => fn(maximized)),
  screenSources: () => ipcRenderer.invoke('screen:sources'),
  chooseScreen: (id) => ipcRenderer.invoke('screen:choose', id),
  nativeScreenAudio: () => ipcRenderer.invoke('screen:native-audio'),
  startScreenAudio: (id, onChunk, onError) => {
    forgetScreenAudio()
    ipcRenderer.on('screen:audio', (_, chunk) => onChunk(chunk))
    ipcRenderer.on('screen:audio-error', (_, message) => onError(message))
    return ipcRenderer.invoke('screen:audio-start', id)
  },
  stopScreenAudio: () => {
    forgetScreenAudio()
    return ipcRenderer.invoke('screen:audio-stop')
  },
  setMuteKey: (accelerator) => ipcRenderer.invoke('hotkey:set', accelerator),
  onMuteKey: (fn) => ipcRenderer.on('hotkey:mute', () => fn()),
  spotifyLogin: (url) => ipcRenderer.invoke('spotify:login', url),
  spotifyToken: (form) => ipcRenderer.invoke('spotify:token', form),
  checkUpdate: () => ipcRenderer.invoke('update:check'),
  installUpdate: () => ipcRenderer.invoke('update:install'),
})
