<script lang="ts">
  import { onMount, tick } from 'svelte'
  import { SvelteSet } from 'svelte/reactivity'
  import { api, auth, hue, type Channel, type Message, type Presence } from './lib/api.svelte'
  import { askPermission, ping, showNotification } from './lib/notify'
  import { settings } from './lib/settings.svelte'
  import { voice } from './lib/voice.svelte'
  import Icon from './lib/Icon.svelte'
  import Settings from './Settings.svelte'
  import Stage from './Stage.svelte'

  const me = auth.session!.user
  const PAGE = 50

  let channels = $state<Channel[]>([])
  let currentId = $state(0)
  let messages = $state<Message[]>([])
  let hasMore = $state(false)
  let presence = $state<Presence[]>([])
  const unread = new SvelteSet<number>()
  let draft = $state('')
  let toast = $state('')
  let creating = $state<Channel['kind'] | null>(null)
  let newName = $state('')
  let showSettings = $state(false)
  let showMembers = $state(true)
  let scroller = $state<HTMLElement>()

  const current = $derived(channels.find((c) => c.id === currentId))
  const members = (id: number) => presence.filter((p) => p.voice === id)

  let ws: WebSocket | undefined
  let stopped = false

  onMount(() => {
    load().catch(notify)
    connect(false)
    if (settings.notify) askPermission()
    return () => {
      stopped = true
      ws?.close()
      voice.leave()
    }
  })

  // O servidor repassa a sala de voz de cada um para a lista lateral.
  $effect(() => {
    const channel = voice.channel
    if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'voice', channel }))
  })

  $effect(() => {
    if (!toast) return
    const t = setTimeout(() => (toast = ''), 4000)
    return () => clearTimeout(t)
  })

  function notify(err: unknown) {
    toast = err instanceof Error ? err.message : String(err)
  }

  async function load() {
    channels = await api<Channel[]>('/api/channels')
    const target = current ?? channels.find((c) => c.kind === 'text')
    if (target?.kind === 'text') await open(target)
  }

  function connect(reconnecting: boolean) {
    const s = auth.session!
    const sock = new WebSocket(s.server.replace(/^http/, 'ws') + '/ws?token=' + encodeURIComponent(s.token))
    ws = sock
    sock.onopen = () => {
      if (voice.channel) sock.send(JSON.stringify({ type: 'voice', channel: voice.channel }))
      if (reconnecting) load().catch(notify)
    }
    sock.onmessage = (e) => handle(JSON.parse(e.data))
    sock.onclose = () => {
      if (!stopped) setTimeout(() => connect(true), 2000)
    }
  }

  function handle(ev: { type: string; [k: string]: any }) {
    if (ev.type === 'presence') presence = ev.users
    else if (ev.type === 'channel') channels.push(ev.channel)
    else if (ev.type === 'message') {
      const m: Message = ev.message
      if (m.channel !== currentId) unread.add(m.channel)
      else if (!messages.length || m.id > messages[messages.length - 1].id) append(m)
      if (m.author !== me.name && (m.channel !== currentId || !document.hasFocus())) announce(m)
    }
  }

  function announce(m: Message) {
    ping()
    if (document.hasFocus()) return
    const channel = channels.find((c) => c.id === m.channel)
    showNotification(`${m.author} em #${channel?.name ?? ''}`, m.body, () => channel && open(channel).catch(notify))
  }

  async function append(m: Message) {
    const stick = !scroller || scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 120 || m.author === me.name
    messages.push(m)
    if (stick) {
      await tick()
      scroller?.scrollTo({ top: scroller.scrollHeight })
    }
  }

  async function open(c: Channel) {
    currentId = c.id
    unread.delete(c.id)
    if (c.kind === 'voice') {
      voice.join(c.id).catch(notify)
      return
    }
    messages = []
    const page = await api<Message[]>(`/api/channels/${c.id}/messages`)
    if (currentId !== c.id) return
    // Mensagens que chegaram pelo WebSocket durante o fetch ficam no fim.
    const last = page.at(-1)?.id ?? 0
    messages = [...page, ...messages.filter((m) => m.id > last)]
    hasMore = page.length === PAGE
    await tick()
    scroller?.scrollTo({ top: scroller.scrollHeight })
  }

  async function older() {
    if (!scroller || !hasMore || scroller.scrollTop > 40) return
    hasMore = false
    const id = currentId
    const page = await api<Message[]>(`/api/channels/${id}/messages?before=${messages[0].id}`)
    if (currentId !== id) return
    const height = scroller.scrollHeight
    messages = [...page, ...messages]
    hasMore = page.length === PAGE
    await tick()
    scroller.scrollTop += scroller.scrollHeight - height
  }

  async function send() {
    const body = draft.trim()
    if (!body) return
    draft = ''
    try {
      await api(`/api/channels/${currentId}/messages`, { method: 'POST', body: { body } })
    } catch (err) {
      draft = body
      notify(err)
    }
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      send()
    }
  }

  function toggleCreating(kind: Channel['kind']) {
    creating = creating === kind ? null : kind
    newName = ''
  }

  async function createChannel(e: SubmitEvent) {
    e.preventDefault()
    try {
      await api('/api/channels', { method: 'POST', body: { name: newName, kind: creating } })
      creating = null
    } catch (err) {
      notify(err)
    }
  }

  async function invite() {
    try {
      const { code } = await api<{ code: string }>('/api/invites', { method: 'POST' })
      await navigator.clipboard.writeText(code)
      toast = `Convite copiado: ${code}`
    } catch (err) {
      notify(err)
    }
  }

  const day = (ts: number) => new Date(ts * 1000).toDateString()
  const newDay = (i: number) => i === 0 || day(messages[i].createdAt) !== day(messages[i - 1].createdAt)
  const continues = (i: number) =>
    !newDay(i) && messages[i - 1].author === messages[i].author && messages[i].createdAt - messages[i - 1].createdAt < 300

  const clock = (ts: number) => new Date(ts * 1000).toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })
  const dayLabel = (ts: number) => new Date(ts * 1000).toLocaleDateString('pt-BR', { day: 'numeric', month: 'long', year: 'numeric' })

  function when(ts: number) {
    const d = new Date(ts * 1000)
    const today = new Date()
    const yesterday = new Date(today.getTime() - 86400000)
    if (d.toDateString() === today.toDateString()) return `Hoje às ${clock(ts)}`
    if (d.toDateString() === yesterday.toDateString()) return `Ontem às ${clock(ts)}`
    return `${d.toLocaleDateString('pt-BR')} ${clock(ts)}`
  }
</script>

<div class="shell" class:no-members={!showMembers || current?.kind !== 'text'}>
  <nav class="rail" aria-label="Servidores">
    <div class="server active" title="Concord"><span>C</span></div>
  </nav>

  <aside class="sidebar">
    <header class="server-name">
      <span>Concord</span>
      <button class="icon-btn" title="Gerar convite" onclick={invite}><Icon name="invite" size={18} /></button>
    </header>

    <div class="channels">
      {#each [{ kind: 'text', label: 'Canais de texto' }, { kind: 'voice', label: 'Canais de voz' }] as const as section}
        <div class="category">
          <span>{section.label}</span>
          <button title="Criar canal" onclick={() => toggleCreating(section.kind)}><Icon name="plus" size={16} /></button>
        </div>
        {#if creating === section.kind}
          <form class="new" onsubmit={createChannel}>
            <!-- svelte-ignore a11y_autofocus -->
            <input bind:value={newName} placeholder="novo-canal" maxlength="32" autofocus onkeydown={(e) => e.key === 'Escape' && toggleCreating(section.kind)} />
          </form>
        {/if}
        {#each channels.filter((c) => c.kind === section.kind) as c (c.id)}
          <button class="channel" class:active={c.id === currentId} class:unread={unread.has(c.id)} onclick={() => open(c).catch(notify)}>
            <Icon name={c.kind === 'text' ? 'hash' : 'volume'} size={20} />
            <span>{c.name}</span>
          </button>
          {#if c.kind === 'voice'}
            {#each members(c.id) as p (p.name)}
              <div class="voice-member" class:speaking={voice.speaking.includes(p.name)}>
                <span class="avatar" style:--h={hue(p.name)} style:--size="24px">{p.name[0]}</span>
                <span>{p.name}</span>
              </div>
            {/each}
          {/if}
        {/each}
      {/each}
    </div>

    {#if voice.channel || voice.connecting}
      <div class="voice-panel">
        <div class="voice-status">
          <strong class:connecting={voice.connecting}>{voice.connecting ? 'Conectando…' : 'Voz conectada'}</strong>
          <small>{channels.find((c) => c.id === voice.channel)?.name ?? ''} / Concord</small>
        </div>
        <button class="icon-btn" class:on={voice.sharing} title={voice.sharing ? 'Parar transmissão' : 'Compartilhar tela'} onclick={() => voice.toggleScreen().catch(notify)}>
          <Icon name="screen" size={20} />
        </button>
        <button class="icon-btn" title="Desconectar" onclick={() => voice.leave()}><Icon name="leave" size={20} /></button>
      </div>
    {/if}

    <footer class="me">
      <span class="avatar online" style:--h={hue(me.name)} style:--size="32px" style:--ring="var(--panel)">{me.name[0]}</span>
      <div class="who">
        <strong>{me.name}</strong>
        <small>Online</small>
      </div>
      <button class="icon-btn" class:off={voice.muted} title={voice.muted ? 'Ativar microfone' : 'Silenciar'} onclick={() => voice.toggleMute().catch(notify)}>
        <Icon name={voice.muted ? 'micOff' : 'mic'} size={20} />
      </button>
      <button class="icon-btn" class:off={voice.deafened} title={voice.deafened ? 'Ativar áudio' : 'Desativar áudio'} onclick={() => voice.toggleDeafen().catch(notify)}>
        <Icon name={voice.deafened ? 'headphonesOff' : 'headphones'} size={20} />
      </button>
      <button class="icon-btn" title="Configurações" onclick={() => (showSettings = true)}><Icon name="gear" size={20} /></button>
    </footer>
  </aside>

  {#if current?.kind === 'voice'}
    <Stage channel={current} members={members(current.id)} onerror={notify} />
  {:else if current}
    <main class="chat">
      <header class="topbar">
        <Icon name="hash" size={22} />
        <h1>{current.name}</h1>
        <button class="icon-btn members-toggle" class:on={showMembers} title={showMembers ? 'Ocultar membros' : 'Mostrar membros'} onclick={() => (showMembers = !showMembers)}>
          <Icon name="users" size={22} />
        </button>
      </header>

      <div class="messages" bind:this={scroller} onscroll={() => older().catch(notify)}>
        <div class="spacer"></div>
        {#if !hasMore}
          <div class="welcome">
            <div class="welcome-icon"><Icon name="hash" size={40} /></div>
            <h2>Bem-vindo a #{current.name}!</h2>
            <p>Este é o começo do canal #{current.name}.</p>
          </div>
        {/if}
        {#each messages as m, i (m.id)}
          {#if newDay(i)}
            <div class="divider"><span>{dayLabel(m.createdAt)}</span></div>
          {/if}
          {#if continues(i)}
            <div class="msg follow">
              <time>{clock(m.createdAt)}</time>
              <p>{m.body}</p>
            </div>
          {:else}
            <div class="msg head">
              <span class="avatar" style:--h={hue(m.author)}>{m.author[0]}</span>
              <div>
                <div class="meta"><strong>{m.author}</strong><time>{when(m.createdAt)}</time></div>
                <p>{m.body}</p>
              </div>
            </div>
          {/if}
        {/each}
        <div class="end"></div>
      </div>

      <form class="composer" onsubmit={(e) => (e.preventDefault(), send())}>
        <textarea bind:value={draft} onkeydown={onKey} rows="1" maxlength="4000" placeholder="Conversar em #{current.name}"></textarea>
      </form>
    </main>

    {#if showMembers}
      <aside class="members">
        <h3>Online · {presence.length}</h3>
        {#each presence as p (p.name)}
          <div class="member">
            <span class="avatar online" style:--h={hue(p.name)} style:--size="32px">{p.name[0]}</span>
            <span class="name">{p.name}</span>
            {#if p.voice}<span class="in-voice" title="Em canal de voz"><Icon name="volume" size={16} /></span>{/if}
          </div>
        {/each}
      </aside>
    {/if}
  {/if}

  {#if toast}<div class="toast" role="status">{toast}</div>{/if}
  {#if showSettings}<Settings onclose={() => (showSettings = false)} oninvite={invite} onerror={notify} />{/if}
</div>

<style>
  .shell {
    height: 100%;
    display: grid;
    grid-template-columns: 72px 240px 1fr 240px;
  }

  .shell.no-members {
    grid-template-columns: 72px 240px 1fr;
  }

  .rail {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding-top: 12px;
    background: var(--rail);
  }

  .server {
    position: relative;
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    border-radius: 16px;
    font-size: 20px;
    font-weight: 700;
    color: #fff;
    background: var(--accent);
  }

  .server.active::before {
    content: '';
    position: absolute;
    left: -12px;
    width: 4px;
    height: 40px;
    border-radius: 0 4px 4px 0;
    background: #fff;
  }

  .sidebar {
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--sidebar);
  }

  .server-name {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 48px;
    padding: 0 8px 0 16px;
    font-weight: 600;
    color: var(--strong);
    box-shadow: 0 1px 0 #0003, 0 2px 0 #0000000d;
  }

  .channels {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
  }

  .category {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 4px 4px 8px;
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.02em;
    color: var(--muted);
  }

  .category:hover {
    color: var(--text);
  }

  .category button {
    display: grid;
    color: var(--muted);
    opacity: 0;
  }

  .category:hover button,
  .category button:focus-visible {
    opacity: 1;
  }

  .category button:hover {
    color: var(--text);
  }

  .new input {
    margin: 2px 0 4px;
    padding: 6px 8px;
  }

  .channel {
    position: relative;
    width: 100%;
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 1px 0;
    padding: 6px 8px;
    border-radius: 4px;
    color: var(--faint);
    text-align: left;
    font-weight: 500;
  }

  .channel span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .channel:hover {
    background: var(--hover);
    color: var(--text);
  }

  .channel.active {
    background: var(--active);
    color: var(--strong);
  }

  .channel.unread {
    color: var(--strong);
    font-weight: 600;
  }

  .channel.unread::before {
    content: '';
    position: absolute;
    left: -8px;
    width: 4px;
    height: 8px;
    border-radius: 0 4px 4px 0;
    background: var(--strong);
  }

  .voice-member {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 8px 4px 34px;
    font-size: 14px;
    color: var(--muted);
  }

  .voice-member .avatar {
    transition: box-shadow 0.15s;
  }

  .voice-member.speaking .avatar {
    box-shadow: 0 0 0 2px var(--sidebar), 0 0 0 4px var(--green);
  }

  .voice-panel {
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 8px;
    background: var(--panel);
    border-bottom: 1px solid #ffffff0f;
  }

  .voice-status {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    padding-left: 4px;
  }

  .voice-status strong {
    font-size: 14px;
    color: var(--green);
  }

  .voice-status strong.connecting {
    color: #f0b232;
  }

  .voice-status small {
    color: var(--muted);
    font-size: 12px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .icon-btn.on {
    color: var(--green);
  }

  .me {
    display: flex;
    align-items: center;
    gap: 2px;
    height: 52px;
    padding: 0 8px;
    background: var(--panel);
  }

  .who {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    margin-left: 8px;
    line-height: 1.2;
  }

  .who strong {
    font-size: 14px;
    color: var(--strong);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .who small {
    font-size: 12px;
    color: var(--muted);
  }

  .chat {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    background: var(--chat);
  }

  .topbar {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 48px;
    padding: 0 16px;
    color: var(--faint);
    box-shadow: 0 1px 0 #0003, 0 2px 0 #0000000d;
  }

  .topbar h1 {
    flex: 1;
    margin: 0;
    font-size: 16px;
    color: var(--strong);
  }

  .members-toggle.on {
    color: var(--strong);
  }

  .messages {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow-y: auto;
    overflow-anchor: none;
  }

  /* Empurra as mensagens para baixo enquanto o canal não enche a tela. */
  .spacer {
    flex: 1;
  }

  .welcome {
    padding: 16px 16px 8px;
  }

  .welcome-icon {
    display: grid;
    place-items: center;
    width: 68px;
    height: 68px;
    border-radius: 50%;
    background: var(--active);
    color: var(--strong);
  }

  .welcome h2 {
    margin: 8px 0;
    font-size: 32px;
    color: var(--strong);
  }

  .welcome p {
    margin: 0;
    color: var(--muted);
  }

  .divider {
    display: flex;
    align-items: center;
    margin: 24px 16px 8px;
    border-top: 1px solid var(--line);
  }

  .divider span {
    margin: -9px auto 0;
    padding: 0 4px;
    font-size: 12px;
    font-weight: 600;
    color: var(--faint);
    background: var(--chat);
  }

  .msg {
    position: relative;
    padding: 2px 48px 2px 72px;
  }

  .msg:hover {
    background: #2e3035;
  }

  .msg p {
    margin: 0;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    color: var(--text);
  }

  .head {
    display: flex;
    margin-top: 17px;
    padding-left: 16px;
    gap: 16px;
  }

  .head .avatar {
    margin-top: 2px;
  }

  .meta {
    display: flex;
    align-items: baseline;
    gap: 8px;
  }

  .meta strong {
    font-weight: 500;
    color: var(--strong);
  }

  .meta time {
    font-size: 12px;
    color: var(--faint);
  }

  .follow time {
    position: absolute;
    left: 0;
    width: 72px;
    top: 4px;
    text-align: center;
    font-size: 11px;
    color: var(--faint);
    visibility: hidden;
  }

  .follow:hover time {
    visibility: visible;
  }

  .end {
    height: 24px;
  }

  .composer {
    margin: 0 16px 24px;
    padding: 0 16px;
    border-radius: var(--radius);
    background: var(--input);
  }

  .composer textarea {
    width: 100%;
    max-height: 50vh;
    padding: 11px 0;
    border: 0;
    outline: none;
    background: none;
    resize: none;
    field-sizing: content;
    line-height: 1.375;
  }

  .composer textarea::placeholder {
    color: var(--faint);
  }

  .members {
    overflow-y: auto;
    padding: 0 8px;
    background: var(--sidebar);
  }

  .members h3 {
    margin: 24px 8px 4px;
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.02em;
    color: var(--muted);
  }

  .member {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 6px 8px;
    border-radius: 4px;
    color: var(--text);
  }

  .member:hover {
    background: var(--hover);
  }

  .member .name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .in-voice {
    display: grid;
    color: var(--green);
  }

  .toast {
    position: fixed;
    left: 50%;
    bottom: 88px;
    z-index: 20;
    transform: translateX(-50%);
    padding: 10px 16px;
    border-radius: var(--radius);
    color: var(--strong);
    background: #111214;
    box-shadow: 0 8px 24px #0008;
  }

  @media (max-width: 960px) {
    .shell,
    .shell.no-members {
      grid-template-columns: 72px 240px 1fr;
    }

    .members {
      display: none;
    }
  }
</style>
