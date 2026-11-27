export type ScreenSource = { id: string; name: string; screen: boolean; thumbnail: string; icon: string | null }

type Desktop = {
  minimize(): void
  toggleMaximize(): void
  close(): void
  isMaximized(): Promise<boolean>
  onMaximized(fn: (maximized: boolean) => void): void
  screenSources(): Promise<ScreenSource[]>
  chooseScreen(id: string): Promise<void>
  nativeScreenAudio(): Promise<boolean>
  startScreenAudio(id: string, onChunk: (pcm: Uint8Array) => void, onError: (message: string) => void): Promise<void>
  stopScreenAudio(): Promise<void>
  checkUpdate(): Promise<{ version: string } | null>
  installUpdate(): Promise<void>
}

// Vem do preload do Electron. No navegador (testes, vite dev) não existe.
export const desktop = (window as { desktop?: Desktop }).desktop
