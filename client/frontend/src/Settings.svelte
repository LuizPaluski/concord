<script lang="ts">
  import { api, auth, logout, people, uploadImage, type Effect, type Profile } from './lib/api.svelte'
  import { desktop } from './lib/desktop'
  import { accelerator } from './lib/hotkey'
  import { askPermission } from './lib/notify'
  import { spotify } from './lib/spotify.svelte'
  import { FRAME_RATES, RESOLUTIONS, THEMES, settings } from './lib/settings.svelte'
  import { voice } from './lib/voice.svelte'
  import Icon from './lib/Icon.svelte'
  import ProfileCard from './ProfileCard.svelte'

  let { onclose, onerror }: { onclose: () => void; onerror: (err: unknown) => void } = $props()

  const me = auth.session!.user
  const EFFECTS: [Effect, string][] = [
    ['', 'Nenhum'],
    ['gradient', 'Gradiente'],
    ['glow', 'Brilho'],
    ['rainbow', 'Arco-íris'],
  ]
  const TABS = { perfil: 'Meu perfil', aparencia: 'Aparência', voz: 'Voz e vídeo', avisos: 'Notificações' } as const

  let tab = $state<keyof typeof TABS>('perfil')
  const BLANK = { avatar: '', banner: '', color: '', color2: '', effect: '' as Effect, bio: '' }
  const saved = (): Profile => ({ ...BLANK, ...people[me.name], ...me })
  let draft = $state(saved())
  let busy = $state(false)
  const dirty = $derived(JSON.stringify(draft) !== JSON.stringify(saved()))
  const denied = 'Notification' in window && Notification.permission === 'denied'

  let capturing = $state(false)
  let keyError = $state('')

  // Enquanto grava, o atalho atual sai do ar; senão o Windows o entrega ao Concord e a tela nunca o vê.
  function startCapture() {
    capturing = true
    keyError = ''
    desktop!.setMuteKey('')
  }

  function stopCapture() {
    if (!capturing) return
    capturing = false
    desktop!.setMuteKey(settings.muteKey)
  }

  async function setKey(key: string) {
    capturing = false
    keyError = ''
    if (await desktop!.setMuteKey(key)) settings.muteKey = key
    else {
      keyError = `${key} já é usado por outro programa. Tente outra combinação.`
      await desktop!.setMuteKey(settings.muteKey)
    }
  }

  function capture(e: KeyboardEvent) {
    if (!capturing) return
    e.preventDefault()
    e.stopPropagation()
    if (e.key === 'Escape') return stopCapture()
    const key = accelerator(e)
    if (key) setKey(key)
  }

  async function pick(e: Event, field: 'avatar' | 'banner', w: number, h: number) {
    const input = e.currentTarget as HTMLInputElement
    const file = input.files?.[0]
    input.value = ''
    if (!file) return
    busy = true
    try {
      draft[field] = await uploadImage(file, w, h)
    } catch (err) {
      onerror(err)
    } finally {
      busy = false
    }
  }

  async function save() {
    busy = true
    try {
      people[me.name] = await api<Profile>('/api/profile', { method: 'POST', body: draft })
    } catch (err) {
      onerror(err)
    } finally {
      busy = false
    }
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="screen" role="dialog" aria-modal="true" aria-label="Configurações">
  <nav>
    <h3>Configurações</h3>
    {#each Object.entries(TABS) as [key, label] (key)}
      <button class:on={tab === key} onclick={() => (tab = key as keyof typeof TABS)}>{label}</button>
    {/each}
    <hr />
    <button class="out" onclick={logout}>Sair <Icon name="logout" size={16} /></button>
  </nav>

  <div class="content">
    <button class="close" title="Fechar (Esc)" aria-label="Fechar" onclick={onclose}><Icon name="close" size={20} /></button>

    {#if tab === 'perfil'}
      <h2>Meu perfil</h2>
      <div class="profile">
        <div class="form">
          <div class="field">
            <strong>Foto</strong>
            <div class="actions">
              <label class="btn">Trocar foto <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" hidden onchange={(e) => pick(e, 'avatar', 256, 256)} /></label>
              {#if draft.avatar}<button class="link" onclick={() => (draft.avatar = '')}>Remover</button>{/if}
            </div>
          </div>
          <div class="field">
            <strong>Banner</strong>
            <div class="actions">
              <label class="btn">Trocar banner <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" hidden onchange={(e) => pick(e, 'banner', 900, 320)} /></label>
              {#if draft.banner}<button class="link" onclick={() => (draft.banner = '')}>Remover</button>{/if}
            </div>
            <small>Sem imagem, o banner usa as cores abaixo. GIF animado funciona.</small>
          </div>
          <div class="field">
            <strong>Cores do perfil</strong>
            <div class="colors">
              <label class="swatch" title="Cor principal"><input type="color" value={draft.color || '#5865f2'} oninput={(e) => (draft.color = e.currentTarget.value)} /></label>
              <label class="swatch" title="Segunda cor"><input type="color" value={draft.color2 || '#eb459e'} oninput={(e) => (draft.color2 = e.currentTarget.value)} /></label>
              {#if draft.color || draft.color2}<button class="link" onclick={() => (draft.color = draft.color2 = '')}>Padrão</button>{/if}
            </div>
            <small>Pintam o nome e o fundo do banner.</small>
          </div>
          <div class="field">
            <strong>Efeito no nome</strong>
            <div class="choices" role="group" aria-label="Efeito no nome">
              {#each EFFECTS as [value, label] (value)}
                <button aria-pressed={draft.effect === value} class:on={draft.effect === value} onclick={() => (draft.effect = value)}>{label}</button>
              {/each}
            </div>
          </div>
          <label class="field">
            <strong>Sobre mim</strong>
            <textarea bind:value={draft.bio} maxlength="190" rows="3" placeholder="Conte algo sobre você"></textarea>
          </label>
        </div>
        <div class="preview">
          <small>Prévia</small>
          <ProfileCard profile={draft} online />
        </div>
      </div>
      {#if dirty}
        <div class="savebar">
          <span>Você tem alterações não salvas.</span>
          <button class="link" onclick={() => (draft = saved())}>Desfazer</button>
          <button class="btn green" disabled={busy} onclick={save}>Salvar</button>
        </div>
      {/if}
    {:else if tab === 'aparencia'}
      <h2>Aparência</h2>
      <h3>Tema</h3>
      <div class="themes">
        {#each Object.entries(THEMES) as [key, label] (key)}
          <button class="theme" data-theme={key} aria-pressed={settings.theme === key} class:on={settings.theme === key} onclick={() => (settings.theme = key as keyof typeof THEMES)}>
            <span class="mini"><i></i><i></i><i></i></span>
            {label}
          </button>
        {/each}
      </div>
    {:else if tab === 'voz'}
      <h2>Voz e vídeo</h2>
      <section>
        <label class="row">
          <span>
            <strong>Supressão de ruído</strong>
            <small>Corta teclado, ventilador e barulho de fundo do seu microfone.</small>
            {#if voice.noiseError}<small class="fail">Não foi possível ativar: {voice.noiseError}</small>{/if}
          </span>
          <input type="checkbox" class="switch" checked={settings.noise} onchange={(e) => voice.setNoise(e.currentTarget.checked).catch(onerror)} />
        </label>
        <label class="row">
          <span>
            <strong>Sons da call</strong>
            <small>Toca um som quando alguém entra ou sai da call e quando você muta ou desmuta.</small>
          </span>
          <input type="checkbox" class="switch" bind:checked={settings.callSound} />
        </label>
      </section>
      {#if desktop}
        <h3>Atalhos</h3>
        <section>
          <div class="row">
            <span>
              <strong>Mutar e desmutar</strong>
              <small>Funciona com o jogo aberto. Use uma combinação, como Ctrl+Shift+M, ou uma tecla F.</small>
              {#if keyError}<small class="fail">{keyError}</small>{/if}
            </span>
            <div class="keys">
              <button class="keycap" class:recording={capturing} onclick={startCapture} onkeydown={capture} onblur={stopCapture}>
                {capturing ? 'Aperte o atalho…' : settings.muteKey || 'Sem atalho'}
              </button>
              {#if settings.muteKey && !capturing}<button class="link" onclick={() => setKey('')}>Remover</button>{/if}
            </div>
          </div>
        </section>
      {/if}
      {#if spotify.available}
        <h3>Spotify</h3>
        <section>
          <div class="row">
            <span>
              <strong>{spotify.connected ? 'Spotify conectado' : 'Spotify não conectado'}</strong>
              <small>Para ouvir junto na call. Precisa de Premium e do app do Spotify aberto no PC.</small>
            </span>
            {#if spotify.connected}
              <button class="link" onclick={() => spotify.logout()}>Desconectar</button>
            {:else}
              <button class="btn" onclick={() => spotify.login().catch(onerror)}>Conectar</button>
            {/if}
          </div>
        </section>
      {/if}
      <h3>Transmissão de tela</h3>
      <section>
        <div class="row">
          <span>
            <strong>Resolução</strong>
            <small>Mais alta deixa texto e detalhes nítidos, mas usa mais internet.</small>
          </span>
          <div class="choices" role="group" aria-label="Resolução">
            {#each Object.keys(RESOLUTIONS).map(Number) as res (res)}
              <button aria-pressed={settings.screenRes === res} class:on={settings.screenRes === res} onclick={() => (settings.screenRes = res as keyof typeof RESOLUTIONS)}>{res}p</button>
            {/each}
          </div>
        </div>
        <div class="row">
          <span>
            <strong>Quadros por segundo</strong>
            <small>60 para jogo e vídeo; 15 ou 30 para código e documento.</small>
          </span>
          <div class="choices" role="group" aria-label="Quadros por segundo">
            {#each FRAME_RATES as fps (fps)}
              <button aria-pressed={settings.screenFps === fps} class:on={settings.screenFps === fps} onclick={() => (settings.screenFps = fps)}>{fps}</button>
            {/each}
          </div>
        </div>
        <label class="row">
          <span>
            <strong>Transmitir o som</strong>
            <small>Envia o áudio do jogo ou do vídeo junto com a tela.</small>
          </span>
          <input type="checkbox" class="switch" bind:checked={settings.screenAudio} />
        </label>
        {#if voice.sharing}<p class="note">As mudanças valem a partir da próxima transmissão.</p>{/if}
      </section>
    {:else}
      <h2>Notificações</h2>
      <section>
        <label class="row">
          <span>
            <strong>Notificações na área de trabalho</strong>
            <small>{denied ? 'Bloqueadas no Windows. Libere nas configurações de notificação do sistema.' : 'Avisa quando chega mensagem com o Concord em segundo plano.'}</small>
          </span>
          <input type="checkbox" class="switch" bind:checked={settings.notify} onchange={() => settings.notify && askPermission()} />
        </label>
        <label class="row">
          <span>
            <strong>Som de mensagem</strong>
            <small>Toca um aviso curto quando chega mensagem nova.</small>
          </span>
          <input type="checkbox" class="switch" bind:checked={settings.sound} />
        </label>
      </section>
    {/if}
  </div>
</div>

<style>
  .screen {
    position: fixed;
    inset: 0;
    z-index: 10;
    display: grid;
    grid-template-columns: minmax(200px, 1fr) minmax(0, 740px) minmax(0, 1fr);
    background: var(--chat);
    animation: zoom 0.18s ease-out;
  }

  :global(.app.desktop) .screen {
    top: 24px;
  }

  nav {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    padding: 60px 8px 20px 20px;
    background: var(--sidebar);
  }

  nav > * {
    width: 192px;
  }

  nav h3 {
    margin: 0 0 4px;
    padding: 0 10px;
  }

  nav button {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 10px;
    margin-bottom: 2px;
    border-radius: 4px;
    color: var(--muted);
    text-align: left;
    font-weight: 500;
  }

  nav button:hover {
    background: var(--hover);
    color: var(--text);
  }

  nav button.on {
    background: var(--active);
    color: var(--strong);
  }

  nav hr {
    margin: 8px 10px;
    width: 172px;
    border: 0;
    border-top: 1px solid var(--line);
  }

  nav .out {
    color: var(--red);
  }

  .content {
    position: relative;
    overflow-y: auto;
    padding: 60px 40px 80px;
  }

  .close {
    position: absolute;
    top: 56px;
    right: 0;
    display: grid;
    place-items: center;
    width: 36px;
    height: 36px;
    border: 2px solid var(--muted);
    border-radius: 50%;
    color: var(--muted);
  }

  .close:hover {
    border-color: var(--strong);
    color: var(--strong);
  }

  h2 {
    margin: 0 0 20px;
    font-size: 20px;
    color: var(--strong);
  }

  h3 {
    margin: 24px 0 8px;
    font-size: 12px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.02em;
    color: var(--muted);
  }

  .profile {
    display: flex;
    gap: 32px;
    flex-wrap: wrap;
  }

  .form {
    flex: 1;
    min-width: 260px;
    display: flex;
    flex-direction: column;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 16px 0;
    border-bottom: 1px solid var(--line);
  }

  .field strong {
    font-size: 12px;
    font-weight: 700;
    text-transform: uppercase;
    color: var(--muted);
  }

  .field small {
    font-size: 13px;
    color: var(--faint);
  }

  .actions,
  .colors {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  label.btn {
    cursor: pointer;
    font-size: 14px;
    padding: 8px 14px;
  }

  .link {
    font-size: 14px;
    color: var(--text);
  }

  .link:hover {
    text-decoration: underline;
  }

  .swatch {
    width: 64px;
    height: 44px;
    overflow: hidden;
    border-radius: 4px;
    cursor: pointer;
    border: 1px solid var(--line);
  }

  .swatch input {
    width: 100%;
    height: 100%;
    padding: 0;
    border: 0;
    background: none;
    cursor: pointer;
    transform: scale(1.6);
  }

  textarea {
    width: 100%;
    padding: 10px;
    border: 0;
    border-radius: 4px;
    background: var(--rail);
    resize: none;
    outline: none;
  }

  .preview {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding-top: 16px;
  }

  .preview small {
    font-size: 12px;
    font-weight: 700;
    text-transform: uppercase;
    color: var(--muted);
  }

  .savebar {
    position: sticky;
    bottom: 0;
    display: flex;
    align-items: center;
    gap: 16px;
    margin-top: 24px;
    padding: 10px 10px 10px 16px;
    border-radius: var(--radius);
    background: var(--rail);
    box-shadow: 0 2px 10px #0004;
    animation: rise 0.2s ease-out;
  }

  .savebar span {
    flex: 1;
    color: var(--strong);
  }

  .green {
    background: var(--green);
  }

  .green:hover {
    background: color-mix(in srgb, var(--green), #000 15%);
  }

  .themes {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 12px;
  }

  .theme {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 8px 8px 10px;
    border-radius: var(--radius);
    border: 2px solid transparent;
    background: var(--sidebar);
    color: var(--text);
    font-size: 14px;
    font-weight: 500;
  }

  .theme.on {
    border-color: var(--accent);
  }

  /* Cada botão recebe as variáveis do próprio tema pelo data-theme. */
  .mini {
    display: grid;
    grid-template-columns: 1fr 2fr 4fr;
    height: 64px;
    overflow: hidden;
    border-radius: 4px;
  }

  .mini i:nth-child(1) {
    background: var(--rail);
  }

  .mini i:nth-child(2) {
    background: var(--sidebar);
  }

  .mini i:nth-child(3) {
    background: linear-gradient(var(--chat) 60%, var(--accent) 60% 72%, var(--chat) 72%);
  }

  section {
    border-bottom: 1px solid var(--line);
  }

  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 12px 0;
    cursor: pointer;
  }

  .row span {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .row strong {
    font-weight: 500;
    color: var(--strong);
  }

  .row small {
    color: var(--muted);
    font-size: 13px;
  }

  .row small.fail {
    color: var(--red);
  }

  .choices {
    display: flex;
    flex: none;
    width: max-content;
    padding: 3px;
    border-radius: 6px;
    background: var(--rail);
  }

  .choices button {
    min-width: 52px;
    padding: 5px 10px;
    border-radius: 4px;
    font-size: 14px;
    font-weight: 500;
    color: var(--muted);
  }

  .choices button:hover {
    color: var(--text);
  }

  .choices button.on {
    color: #fff;
    background: var(--accent);
  }

  .keys {
    display: flex;
    align-items: center;
    gap: 12px;
    flex: none;
  }

  .keycap {
    min-width: 140px;
    padding: 8px 12px;
    border-radius: 4px;
    font-size: 14px;
    font-weight: 600;
    font-family: ui-monospace, Consolas, monospace;
    color: var(--strong);
    background: var(--rail);
    border: 1px solid var(--line);
  }

  .keycap.recording {
    color: var(--red);
    border-color: var(--red);
  }

  .note {
    margin: 0 0 12px;
    font-size: 13px;
    color: var(--muted);
  }

  .switch {
    appearance: none;
    flex: none;
    position: relative;
    width: 40px;
    height: 24px;
    padding: 0;
    border-radius: 12px;
    background: var(--faint);
    cursor: pointer;
    transition: background 0.15s;
  }

  .switch::after {
    content: '';
    position: absolute;
    top: 3px;
    left: 3px;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: #fff;
    transition: transform 0.15s;
  }

  .switch:checked {
    background: var(--green);
  }

  .switch:checked::after {
    transform: translateX(16px);
  }

  @keyframes zoom {
    from {
      transform: scale(1.04);
      opacity: 0;
    }
  }

  @keyframes rise {
    from {
      transform: translateY(12px);
      opacity: 0;
    }
  }

  @media (max-width: 760px) {
    .screen {
      grid-template-columns: 180px 1fr;
    }

    nav {
      padding-left: 8px;
    }

    nav > * {
      width: 164px;
    }

    .close {
      right: 12px;
      top: 12px;
    }
  }
</style>
