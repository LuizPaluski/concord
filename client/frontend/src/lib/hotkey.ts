const NAMED: Record<string, string> = {
  Space: 'Space',
  Backquote: '`',
  Minus: '-',
  Equal: '=',
  BracketLeft: '[',
  BracketRight: ']',
  Backslash: '\\',
  Semicolon: ';',
  Quote: "'",
  Comma: ',',
  Period: '.',
  Slash: '/',
  Insert: 'Insert',
  Delete: 'Delete',
  Home: 'Home',
  End: 'End',
  PageUp: 'PageUp',
  PageDown: 'PageDown',
  ArrowUp: 'Up',
  ArrowDown: 'Down',
  ArrowLeft: 'Left',
  ArrowRight: 'Right',
  Pause: 'Pause',
  ScrollLock: 'Scrolllock',
  NumpadAdd: 'numadd',
  NumpadSubtract: 'numsub',
  NumpadMultiply: 'nummult',
  NumpadDivide: 'numdiv',
  NumpadDecimal: 'numdec',
}

// Traduz a tecla física (e.code) para o nome que o globalShortcut do Electron entende.
function keyName(code: string) {
  if (/^Key[A-Z]$/.test(code)) return code.slice(3)
  if (/^Digit\d$/.test(code)) return code.slice(5)
  if (/^F\d{1,2}$/.test(code)) return code
  if (/^Numpad\d$/.test(code)) return 'num' + code.slice(6)
  return NAMED[code] ?? null
}

// Monta o atalho a partir do que foi apertado. Devolve null enquanto só há modificador
// e recusa letra, número ou espaço sozinhos, que roubariam a digitação em todo o Windows.
export function accelerator(e: KeyboardEvent): string | null {
  const key = keyName(e.code)
  if (!key) return null
  const mods = [e.ctrlKey && 'Ctrl', e.altKey && 'Alt', e.shiftKey && 'Shift', e.metaKey && 'Super'].filter(Boolean) as string[]
  if (!mods.length && /^([A-Z0-9]|Space)$/.test(key)) return null
  return [...mods, key].join('+')
}
