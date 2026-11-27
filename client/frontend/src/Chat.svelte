<script lang="ts">
  import { onMount, tick } from 'svelte'
  import { SvelteSet } from 'svelte/reactivity'
  import { api, auth, hue, logout, type Channel, type Message, type Presence } from './lib/api.svelte'
  import { voice } from './lib/voice.svelte'
  import Icon from './lib/Icon.svelte'
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
  let list = $state<HTMLElement>()

  const current = $derived(channels.find((c) => c.id === currentId))
  const members = (id: number) => presence.filter((p) => p.voice === id)

  let ws: WebSocket | undefined
  let stopped = false

  onMount(() => {
    load().catch(notify)
    connect(false)
    return () => {
      stopped = true
      ws?.close()
      voice.leave()
    }
  })

  // Avisa o servidor em que sala de voz estou, para todos verem na lista.
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
    }
  }

  async function append(m: Message) {
    const stick = !list || list.scrollHeight - list.scrollTop - list.clientHeight < 120 || m.author === me.name
    messages.push(m)
    if (stick) {
      await tick()
      list?.scrollTo({ top: list.scrollHeight })
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
    list?.scrollTo({ top: list.scrollHeight })
  }

  async function older() {
    if (!list || !hasMore || list.scrollTop > 40) return
    hasMore = false
    const id = currentId
    const page = await api<Message[]>(`/api/channels/${id}/messages?before=${messages[0].id}`)
    if (currentId !== id) return
    const height = list.scrollHeight
    messages = [...page, ...messages]
    hasMore = page.length === PAGE
    await tick()
    list.scrollTop += list.scrollHeight - height
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

  async function createChannel(e: SubmitEvent) {
    e.preventDefault()
    try {
      await api('/api/channels', { method: 'POST', body: { name: newName, kind: creating } })
      creating = null
      newName = ''
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

  const continues = (i: number) =>
    i > 0 && messages[i - 1].author === messages[i].author && messages[i].createdAt - messages[i - 1].createdAt < 300

  function when(ts: number) {
    const d = new Date(ts * 1000)
    const time = d.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })
    return d.toDateString() === new Date().toDateString() ? `hoje às ${time}` : `${d.toLocaleDateString('pt-BR')} ${time}`
  }
</script>

<div class="shell">
  <aside class="sidebar">
    <header class="brand">Concord</header>

    <nav>
      {#each [{ kind: 'text', label: 'Texto' }, { kind: 'voice', label: 'Voz' }] as const as section}
        <div class="section">
          <span>{section.label}</span>
          <button class="icon-btn small" title="Novo canal" onclick={() => (creating = creating === section.kind ? null : section.kind)}>
            <Icon name="plus" size={15} />
          </button>
        </div>
        {#if creating === section.kind}
          <form class="new" onsubmit={createChannel}>
            <!-- svelte-ignore a11y_autofocus -->
            <input bind:value={newName} placeholder="nome do canal" maxlength="32" autofocus onkeydown={(e) => e.key === 'Escape' && (creating = null)} />
          </form>
        {/if}
        {#each channels.filter((c) => c.kind === section.kind) as c (c.id)}
          <button class="channel" class:active={c.id === currentId} class:unread={unread.has(c.id)} onclick={() => open(c).catch(notify)}>
            <Icon name={c.kind === 'text' ? 'hash' : 'volume'} size={17} />
            <span>{c.name}</span>
          </button>
          {#if c.kind === 'voice'}
            {#each members(c.id) as p (p.name)}
              <div class="voice-member" class:speaking={voice.speaking.includes(p.name)}>
                <span class="avatar tiny" style:--h={hue(p.name)}>{p.name[0]}</span>
                {p.name}
              </div>
            {/each}
          {/if}
        {/each}
      {/each}
    </nav>

    {#if voice.channel || voice.connecting}
      <div class="voice-bar">
        <div>
          <strong>{voice.connecting ? 'Conectando…' : 'Voz conectada'}</strong>
          <small>{channels.find((c) => c.id === voice.channel)?.name ?? ''}</small>
        </div>
        <button class="icon-btn" class:on={voice.sharing} title="Compartilhar tela" onclick={() => voice.toggleScreen().catch(notify)}>
          <Icon name="screen" />
        </button>
        <button class="icon-btn off" title="Sair da voz" onclick={() => voice.leave()}><Icon name="leave" /></button>
      </div>
    {/if}

    <footer class="me">
      <span class="avatar" style:--h={hue(me.name)}>{me.name[0]}</span>
      <strong>{me.name}</strong>
      <button class="icon-btn" class:off={voice.muted} title={voice.muted ? 'Ativar microfone' : 'Silenciar microfone'} onclick={() => voice.toggleMute().catch(notify)}>
        <Icon name={voice.muted ? 'micOff' : 'mic'} />
      </button>
      <button class="icon-btn" class:off={voice.deafened} title={voice.deafened ? 'Ativar áudio' : 'Desativar áudio'} onclick={() => voice.toggleDeafen().catch(notify)}>
        <Icon name={voice.deafened ? 'headphonesOff' : 'headphones'} />
      </button>
      <button class="icon-btn" title="Gerar convite" onclick={invite}><Icon name="invite" /></button>
      <button class="icon-btn" title="Sair" onclick={logout}><Icon name="logout" /></button>
    </footer>
  </aside>

  {#if current?.kind === 'voice'}
    <Stage channel={current} members={members(current.id)} onerror={notify} />
  {:else if current}
    <main class="chat">
      <header class="topbar"><Icon name="hash" /> {current.name}</header>

      <div class="messages" bind:this={list} onscroll={() => older().catch(notify)}>
        {#if !hasMore}
          <div class="start">
            <h2>Bem-vindo a #{current.name}</h2>
            <p>Esse é o começo do canal.</p>
          </div>
        {/if}
        {#each messages as m, i (m.id)}
          {#if continues(i)}
            <p class="body follow">{m.body}</p>
          {:else}
            <article>
              <span class="avatar" style:--h={hue(m.author)}>{m.author[0]}</span>
              <div>
                <header><strong>{m.author}</strong> <time>{when(m.createdAt)}</time></header>
                <p class="body">{m.body}</p>
              </div>
            </article>
          {/if}
        {/each}
      </div>

      <form class="composer" onsubmit={(e) => (e.preventDefault(), send())}>
        <textarea bind:value={draft} onkeydown={onKey} rows="1" maxlength="4000" placeholder="Conversar em #{current.name}"></textarea>
        <button class="icon-btn" title="Enviar" disabled={!draft.trim()}><Icon name="send" /></button>
      </form>
    </main>

    <aside class="online">
      <div class="section"><span>Online · {presence.length}</span></div>
      {#each presence as p (p.name)}
        <div class="person">
          <span class="avatar small" style:--h={hue(p.name)}>{p.name[0]}</span>
          {p.name}
        </div>
      {/each}
    </aside>
  {/if}

  {#if toast}<div class="toast" role="status">{toast}</div>{/if}
</div>

<style>
  .shell {
    height: 100%;
    display: grid;
    grid-template-columns: 250px 1fr auto;
  }

  .sidebar {
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--panel);
    border-right: 1px solid var(--line);
  }

  .brand {
    padding: 16px 18px;
    font-weight: 700;
    font-size: 17px;
    letter-spacing: -0.01em;
    border-bottom: 1px solid var(--line);
  }

  nav {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
  }

  .section {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 8px 6px;
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
  }

  .small {
    width: 24px;
    height: 24px;
  }

  .new input {
    width: 100%;
    margin: 2px 0 6px;
    padding: 7px 10px;
  }

  .channel {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 7px 10px;
    border-radius: 8px;
    color: var(--muted);
    text-align: left;
  }

  .channel:hover {
    background: var(--hover);
    color: var(--text);
  }

  .channel.active {
    background: var(--hover);
    color: var(--text);
  }

  .channel.unread {
    color: var(--text);
    font-weight: 600;
  }

  .voice-member {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 10px 4px 34px;
    font-size: 14px;
    color: var(--muted);
  }

  .avatar.tiny {
    width: 22px;
    height: 22px;
    font-size: 11px;
    transition: box-shadow 0.15s;
  }

  .voice-member.speaking .avatar {
    box-shadow: 0 0 0 2px var(--panel), 0 0 0 4px var(--speak);
  }

  .voice-bar {
    display: flex;
    align-items: center;
    gap: 4px;
    margin: 8px;
    padding: 10px 10px 10px 12px;
    border-radius: var(--radius);
    background: var(--panel-2);
  }

  .voice-bar div {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .voice-bar strong {
    color: var(--speak);
    font-size: 13px;
  }

  .voice-bar small {
    color: var(--muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .icon-btn.on {
    color: var(--accent);
  }

  .me {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 10px;
    background: var(--panel-2);
  }

  .me strong {
    flex: 1;
    margin-left: 6px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .me .avatar {
    width: 32px;
    height: 32px;
  }

  .chat {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }

  .topbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 14px 20px;
    font-weight: 600;
    border-bottom: 1px solid var(--line);
  }

  .messages {
    flex: 1;
    overflow-y: auto;
    padding: 16px 0;
  }

  .start {
    padding: 24px 20px 16px;
  }

  .start h2 {
    margin: 0 0 4px;
    font-size: 24px;
  }

  .start p {
    margin: 0;
    color: var(--muted);
  }

  article {
    display: flex;
    gap: 14px;
    margin-top: 14px;
    padding: 2px 20px;
  }

  article:hover,
  .follow:hover {
    background: #ffffff05;
  }

  article header {
    display: flex;
    align-items: baseline;
    gap: 8px;
  }

  time {
    font-size: 12px;
    color: var(--muted);
  }

  .body {
    margin: 2px 0 0;
    line-height: 1.45;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .follow {
    padding: 1px 20px 1px 70px;
  }

  .composer {
    display: flex;
    align-items: flex-end;
    gap: 8px;
    margin: 0 20px 20px;
    padding: 6px 6px 6px 14px;
    background: var(--panel-2);
    border: 1px solid var(--line);
    border-radius: 12px;
  }

  .composer:focus-within {
    border-color: #5eead466;
  }

  .composer textarea {
    flex: 1;
    max-height: 200px;
    padding: 7px 0;
    border: 0;
    background: none;
    resize: none;
    field-sizing: content;
  }

  .composer button:disabled {
    opacity: 0.4;
  }

  .online {
    width: 220px;
    padding: 8px;
    overflow-y: auto;
    background: var(--panel);
    border-left: 1px solid var(--line);
  }

  .person {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 8px;
    border-radius: 8px;
  }

  .avatar.small {
    width: 30px;
    height: 30px;
    font-size: 13px;
  }

  .toast {
    position: fixed;
    left: 50%;
    bottom: 24px;
    transform: translateX(-50%);
    padding: 10px 16px;
    border-radius: 10px;
    background: var(--panel-2);
    border: 1px solid var(--line);
    box-shadow: 0 12px 40px #0009;
  }

  @media (max-width: 900px) {
    .online {
      display: none;
    }
  }
</style>
