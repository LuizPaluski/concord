const { app, BrowserWindow, desktopCapturer, globalShortcut, ipcMain, net, protocol, session, shell } = require('electron')
const { autoUpdater } = require('electron-updater')
const { spawn } = require('node:child_process')
const os = require('node:os')
const path = require('node:path')
const { pathToFileURL } = require('node:url')

const UI = path.join(__dirname, '..', 'frontend', 'dist')
const ALLOWED = ['media', 'notifications', 'clipboard-sanitized-write', 'display-capture']
// O loopback por processo do Windows (usado pelo concord-audio) existe do build 20348 em diante.
const PROCESS_LOOPBACK = Number(os.release().split('.')[2]) >= 20348
const AUDIO_HELPER = app.isPackaged ? path.join(process.resourcesPath, 'concord-audio.exe') : path.join(__dirname, '..', 'build', 'concord-audio.exe')

// Fonte escolhida no seletor do app, entregue ao próximo getDisplayMedia da página.
let chosenSource = null
let audioHelper = null
let win

// Uma janela só: abrir de novo foca a existente, e a atualização não esbarra num .exe preso.
if (!app.requestSingleInstanceLock()) app.exit(0)

// Sem isso a notificação sai com o nome do Electron em vez de Concord.
app.setAppUserModelId('com.luizpaluski.concord')

// O RNNoise e o worklet de áudio são carregados por fetch, que não funciona em file://.
protocol.registerSchemesAsPrivileged([{ scheme: 'app', privileges: { standard: true, secure: true, supportFetchAPI: true } }])

autoUpdater.autoDownload = false

// O som da tela chega a cada ~10 ms; com a janela já destruída, send lançaria exceção.
function send(channel, ...args) {
  if (win && !win.isDestroyed()) win.webContents.send(channel, ...args)
}

function createWindow() {
  win = new BrowserWindow({
    width: 1200,
    height: 780,
    minWidth: 760,
    minHeight: 500,
    frame: false,
    show: false,
    backgroundColor: '#1e1f22',
    webPreferences: { preload: path.join(__dirname, 'preload.js'), sandbox: true },
  })
  win.once('ready-to-show', () => win.show())
  win.on('maximize', () => send('window:maximized', true))
  win.on('unmaximize', () => send('window:maximized', false))
  win.on('closed', stopScreenAudio)
  // Página recarregada ou travada perde o fio do áudio; o concord-audio não pode ficar órfão.
  win.webContents.on('did-start-loading', stopScreenAudio)
  win.webContents.on('render-process-gone', stopScreenAudio)
  // Link clicado no chat abre no navegador, nunca dentro do app.
  win.webContents.setWindowOpenHandler(({ url }) => {
    if (/^https?:\/\//.test(url)) shell.openExternal(url)
    return { action: 'deny' }
  })
  win.webContents.on('will-navigate', (e) => e.preventDefault())
  win.loadURL('app://concord/index.html')
}

function stopScreenAudio() {
  // Fechar a entrada é o sinal para o concord-audio encerrar sozinho.
  audioHelper?.stdin.end()
  audioHelper = null
}

app.on('second-instance', () => {
  if (!win) return
  if (win.isMinimized()) win.restore()
  win.focus()
})

app.whenReady().then(() => {
  protocol.handle('app', (req) => {
    const file = path.join(UI, decodeURIComponent(new URL(req.url).pathname))
    if (!file.startsWith(UI)) return new Response('', { status: 403 })
    return net.fetch(pathToFileURL(file).toString())
  })

  session.defaultSession.setPermissionRequestHandler((_, permission, done) => done(ALLOWED.includes(permission)))
  session.defaultSession.setPermissionCheckHandler((_, permission) => ALLOWED.includes(permission))

  // Captura pela API do Electron: sem a barra "está compartilhando sua tela" do Chromium.
  session.defaultSession.setDisplayMediaRequestHandler(async (req, done) => {
    const id = chosenSource
    chosenSource = null
    const sources = await desktopCapturer.getSources({ types: ['screen', 'window'], thumbnailSize: { width: 0, height: 0 } })
    const video = sources.find((s) => s.id === id)
    if (!video) return done({})
    // Com loopback por processo, o som vem do concord-audio. Sem ele (Windows antigo),
    // sobra o loopback do Electron, que falha com saída USB (electron/electron#41697).
    done(req.audioRequested && !PROCESS_LOOPBACK ? { video, audio: 'loopback' } : { video })
  })

  createWindow()
})

app.on('window-all-closed', () => app.quit())
app.on('before-quit', stopScreenAudio)

ipcMain.on('window:minimize', () => win.minimize())
ipcMain.on('window:toggle-maximize', () => (win.isMaximized() ? win.unmaximize() : win.maximize()))
ipcMain.on('window:close', () => win.close())
ipcMain.handle('window:is-maximized', () => win.isMaximized())

ipcMain.handle('screen:sources', async () => {
  const sources = await desktopCapturer.getSources({ types: ['screen', 'window'], thumbnailSize: { width: 320, height: 180 }, fetchWindowIcons: true })
  return sources.map((s) => ({
    id: s.id,
    name: s.name,
    screen: s.id.startsWith('screen:'),
    thumbnail: s.thumbnail.toDataURL(),
    icon: s.appIcon?.toDataURL() ?? null,
  }))
})

ipcMain.handle('screen:choose', (_, id) => {
  chosenSource = id
})

ipcMain.handle('screen:native-audio', () => PROCESS_LOOPBACK)

// PCM 16 bits estéreo: cada quadro tem 4 bytes.
const FRAME_BYTES = 4

ipcMain.handle('screen:audio-start', (_, id) => {
  stopScreenAudio()
  // Janela: só o som do programa dela. Tela inteira: tudo menos o próprio Concord,
  // senão quem assiste ouve a call de volta.
  const [kind, handle] = id.split(':')
  const args = kind === 'window' ? ['window', handle] : ['exclude', String(process.pid)]
  const helper = spawn(AUDIO_HELPER, args, { windowsHide: true })
  audioHelper = helper
  let rest = Buffer.alloc(0)
  helper.stdout.on('data', (chunk) => {
    // Só quadros inteiros atravessam, senão o estéreo sai trocado.
    const pending = Buffer.concat([rest, chunk])
    const whole = pending.length - (pending.length % FRAME_BYTES)
    rest = pending.subarray(whole)
    if (whole) send('screen:audio', pending.subarray(0, whole))
  })
  let stderr = ''
  helper.stderr.on('data', (text) => (stderr += text))
  helper.on('exit', (code) => {
    if (code && helper === audioHelper) send('screen:audio-error', stderr.trim() || `código ${code}`)
  })
  helper.on('error', (err) => send('screen:audio-error', err.message))
})

ipcMain.handle('screen:audio-stop', stopScreenAudio)

// Atalho global de mutar: funciona com o jogo em primeiro plano. O Windows entrega a
// combinação só para o Concord, então ela deixa de chegar nos outros programas.
ipcMain.handle('hotkey:set', (_, accelerator) => {
  globalShortcut.unregisterAll()
  if (!accelerator) return true
  try {
    return globalShortcut.register(accelerator, () => send('hotkey:mute'))
  } catch {
    return false
  }
})

app.on('will-quit', () => globalShortcut.unregisterAll())

ipcMain.handle('update:check', async () => {
  if (!app.isPackaged) return null
  const check = await autoUpdater.checkForUpdates()
  return check?.isUpdateAvailable ? { version: check.updateInfo.version } : null
})

ipcMain.handle('update:install', async () => {
  await autoUpdater.downloadUpdate()
  // Instala calado e abre o Concord de novo no fim.
  autoUpdater.quitAndInstall(true, true)
})
