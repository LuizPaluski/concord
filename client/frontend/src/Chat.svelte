<script lang="ts">
  import { onMount, tick } from 'svelte'
  import { SvelteSet } from 'svelte/reactivity'
  import { api, auth, loadPeople, mediaUrl, people, uploadFile, type Channel, type Guild, type Message, type Presence } from './lib/api.svelte'
  import { askPermission, gunshot, ping, showNotification } from './lib/notify'
  import { settings } from './lib/settings.svelte'
  import { lastCall, voice } from './lib/voice.svelte'
  import Attachment from './lib/Attachment.svelte'
  import Avatar from './lib/Avatar.svelte'
  import Icon from './lib/Icon.svelte'
  import Name from './lib/Name.svelte'
  import AddServer from './AddServer.svelte'
  import ProfileCard from './ProfileCard.svelte'
  import ServerSettings from './ServerSettings.svelte'
  import Settings from './Settings.svelte'
  import SourcePicker from './SourcePicker.svelte'
  import Stage from './Stage.svelte'
  import StreamPreview from './StreamPreview.svelte'
  import VoiceUserMenu from './VoiceUserMenu.svelte'
  import VoiceStatus from './lib/VoiceStatus.svelte'

  const me = auth.session!.user
  const PAGE = 50
  const SERVER_KEY = 'concord.server-id'

  let guilds = $state<Guild[]>([])
  let serverId = $state(Number(localStorage.getItem(SERVER_KEY)) || 0)
  let channels = $state<Channel[]>([])
  let memberIds = $state<number[]>([])
  let currentId = $state(0)
  let messages = $state<Message[]>([])
  let hasMore = $state(false)
  let presence = $state<Presence[]>([])
  const unread = new SvelteSet<number>()
  const unreadServers = new SvelteSet<number>()
  let draft = $state('')
  let pending = $state<{ file: File; preview: string } | null>(null)
  let sending = $state(false)
  let viewing = $state('')
  const MAX_FILE = 100 * 1024 * 1024
  let toast = $state('')
  let creating = $state<Channel['kind'] | null>(null)
  let newName = $state('')
  let modal = $state<'settings' | 'server' | 'add' | null>(null)
  let menu = $state(false)
  let card = $state<{ name: string; x: number; y: number } | null>(null)
  let listenMenu = $state<{ name: string; x: number; y: number } | null>(null)
  let preview = $state<{ name: string; x: number; y: number } | null>(null)
  let showMembers = $state(true)
  let scroller = $state<HTMLElement>()
  // Rótulo da call atual, que pode ser de outro grupo que não o aberto.
  let voiceWhere = $state('')
  let voiceServer = 0

  const guild = $derived(guilds.find((g) => g.id === serverId))
  const owner = $derived(guild?.owner === me.id)
  const current = $derived(channels.find((c) => c.id === currentId))
  const online = $derived(new Set(presence.map((p) => p.name)))
  const roster = $derived(
    memberIds
      .map((id) => Object.values(people).find((p) => p.id === id)?.name)
      .filter((n): n is string => !!n)
      .sort((a, b) => a.localeCompare(b)),
  )
  const members = (id: number) => presence.filter((p) => p.voice === id)

  let ws: WebSocket | undefined
  let stopped = false

  onMount(() => {
    load().then(rejoin).catch(notify)
    connect(false)
    if (settings.notify) askPermission()
    return () => {
      stopped = true
      ws?.close()
      voice.leave()
    }
  })

  // O servidor repassa a sala de voz e o estado de cada um para a lista lateral.
  const voiceState = () => JSON.stringify({ type: 'voice', channel: voice.channel || voice.target, muted: voice.muted || voice.deafened, deafened: voice.deafened, sharing: voice.sharing })
  $effect(() => {
    const state = voiceState()
    if (ws?.readyState === WebSocket.OPEN) ws.send(state)
  })

  $effect(() => {
    if (!voice.notice) return
    toast = voice.notice
    voice.notice = ''
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
    const [list] = await Promise.all([api<Guild[]>('/api/servers'), loadPeople()])
    guilds = list
    const target = guilds.find((g) => g.id === serverId) ?? guilds[0]
    // Depois de reconectar, recarrega o canal aberto para trazer o que chegou no meio tempo.
    if (target) return openServer(target.id, currentId, true)
    serverId = currentId = 0
    channels = []
  }

  // Volta para a call em que eu estava quando o app fechou.
  async function rejoin() {
    const last = lastCall()
    if (!last || voice.channel || !guilds.some((g) => g.id === last.server)) return
    const chans = last.server === serverId ? channels : await api<Channel[]>(`/api/servers/${last.server}/channels`)
    if (!chans.some((c) => c.id === last.channel && c.kind === 'voice')) return
    voiceWhere = last.label
    voiceServer = last.server
    await voice.join(last.channel, last)
  }

  async function openServer(id: number, keep = 0, reload = false) {
    if (id !== serverId) (currentId = 0), (messages = [])
    serverId = id
    localStorage.setItem(SERVER_KEY, String(id))
    unreadServers.delete(id)
    menu = false
    const [chans, ids] = await Promise.all([api<Channel[]>(`/api/servers/${id}/channels`), api<number[]>(`/api/servers/${id}/members`)])
    if (serverId !== id) return
    channels = chans
    memberIds = ids
    const firstVoice = chans.find((c) => c.kind === 'voice')
    if (firstVoice) voice.prefetch(firstVoice.id).catch(() => {})
    const target = channels.find((c) => c.id === keep) ?? channels.find((c) => c.kind === 'text')
    if (target && ((reload && target.kind === 'text') || target.id !== currentId)) await open(target)
    else if (!target) currentId = 0
  }

  function connect(reconnecting: boolean) {
    const s = auth.session!
    const sock = new WebSocket(s.server.replace(/^http/, 'ws') + '/ws?token=' + encodeURIComponent(s.token))
    ws = sock
    sock.onopen = () => {
      if (voice.channel) sock.send(voiceState())
      if (reconnecting) load().catch(notify)
    }
    sock.onmessage = (e) => handle(JSON.parse(e.data))
    sock.onclose = () => {
      if (!stopped) setTimeout(() => connect(true), 2000)
    }
  }

  async function serverChanged(id: number) {
    guilds = await api<Guild[]>('/api/servers')
    await loadPeople()
    if (!guilds.some((g) => g.id === id)) {
      if (voiceServer === id) voice.leave()
      if (id === serverId) await load()
      return
    }
    const chans = id === serverId ? (await openServer(id, currentId), channels) : voiceServer === id ? await api<Channel[]>(`/api/servers/${id}/channels`) : null
    if (chans && voiceServer === id && voice.channel && !chans.some((c) => c.id === voice.channel)) voice.leave()
  }

  function handle(ev: { type: string; [k: string]: any }) {
    if (ev.type === 'presence') presence = ev.users
    else if (ev.type === 'server') serverChanged(ev.id).catch(notify)
    else if (ev.type === 'profile') people[ev.user.name] = ev.user
    else if (ev.type === 'shot' && !voice.deafened) gunshot()
    else if (ev.type === 'release') window.dispatchEvent(new Event('concord:release'))
    else if (ev.type === 'message') {
      const m: Message = ev.message
      if (ev.server !== serverId) unreadServers.add(ev.server)
      else if (m.channel !== currentId) unread.add(m.channel)
      else if (!messages.length || m.id > messages[messages.length - 1].id) append(m)
      if (m.author !== me.name && (m.channel !== currentId || !document.hasFocus())) announce(m, ev.server)
    }
  }

  function announce(m: Message, server: number) {
    ping()
    if (document.hasFocus()) return
    const channel = channels.find((c) => c.id === m.channel)
    const where = channel ? `#${channel.name}` : (guilds.find((g) => g.id === server)?.name ?? '')
    const text = m.body || (m.file?.mime.startsWith('video/') ? 'Enviou um vídeo' : 'Enviou uma foto')
    showNotification(`${m.author} em ${where}`, text, () => openServer(server, m.channel).catch(notify))
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
      voiceWhere = `${c.name} / ${guild?.name ?? ''}`
      voiceServer = serverId
      voice.join(c.id, { server: serverId, channel: c.id, label: voiceWhere }).catch(notify)
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

  function attach(file: File | undefined) {
    if (!file) return
    if (!/^(image\/(png|jpeg|gif|webp)|video\/(mp4|webm))$/.test(file.type)) return notify('Só foto (PNG, JPG, GIF, WebP) ou vídeo (MP4, WebM).')
    if (file.size > MAX_FILE) return notify('Arquivo maior que 100 MB.')
    if (pending) URL.revokeObjectURL(pending.preview)
    pending = { file, preview: URL.createObjectURL(file) }
  }

  // Foto e vídeo crescem ao carregar; quem estava no fim da conversa continua no fim.
  function keepBottom(e: Event) {
    const el = e.target as HTMLElement
    if (scroller && scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight - el.clientHeight < 150) scroller.scrollTo({ top: scroller.scrollHeight })
  }

  function unattach() {
    if (pending) URL.revokeObjectURL(pending.preview)
    pending = null
  }

  async function send() {
    const body = draft.trim()
    if ((!body && !pending) || sending) return
    sending = true
    try {
      const file = pending ? (await uploadFile(pending.file)).id : undefined
      await api(`/api/channels/${currentId}/messages`, { method: 'POST', body: { body, file } })
      draft = ''
      unattach()
    } catch (err) {
      notify(err)
    } finally {
      sending = false
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
      const c = await api<Channel>(`/api/servers/${serverId}/channels`, { method: 'POST', body: { name: newName, kind: creating } })
      creating = null
      if (c.kind === 'text') {
        channels.push(c)
        await open(c)
      }
    } catch (err) {
      notify(err)
    }
  }

  async function invite() {
    menu = false
    try {
      const { code } = await api<{ code: string }>(`/api/servers/${serverId}/invites`, { method: 'POST' })
      await navigator.clipboard.writeText(code)
      toast = `Convite para ${guild?.name} copiado: ${code}`
    } catch (err) {
      notify(err)
    }
  }

  async function leave() {
    menu = false
    if (!guild || !confirm(`Sair de ${guild.name}? Para voltar, só com convite novo.`)) return
    await api(`/api/servers/${guild.id}/leave`, { method: 'POST' }).catch(notify)
  }

  let lastShot = 0
  function shoot() {
    if (Date.now() - lastShot < 1500) return
    lastShot = Date.now()
    ws?.send(JSON.stringify({ type: 'shot' }))
  }

  function showPreview(e: MouseEvent, name: string) {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    preview = { name, x: r.right, y: r.top - 40 }
  }

  function showListen(e: MouseEvent, name: string) {
    e.preventDefault()
    e.stopPropagation()
    card = null
    listenMenu = name === me.name ? null : { name, x: e.clientX, y: e.clientY }
  }

  function showCard(e: MouseEvent, name: string) {
    e.stopPropagation()
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    // Abre ao lado do clique sem sair da janela; o cartão tem ~300x400.
    const x = r.right + 316 < innerWidth ? r.right + 16 : Math.max(16, r.left - 316)
    card = card?.name === name ? null : { name, x, y: Math.min(r.top, innerHeight - 420) }
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

  const initials = (name: string) =>
    name
      .split(/\s+/)
      .slice(0, 3)
      .map((w) => w[0])
      .join('')
</script>

<svelte:window onclick={() => ((card = listenMenu = null), (menu = false))} onkeydown={(e) => e.key === 'Escape' && ((card = listenMenu = null), (menu = false))} />

<div class="shell" class:no-members={!showMembers || current?.kind !== 'text'}>
  <nav class="rail" aria-label="Grupos">
    {#each guilds as g (g.id)}
      <button class="server" class:active={g.id === serverId} class:unread={unreadServers.has(g.id)} aria-label={g.name} onclick={() => openServer(g.id).catch(notify)}>
        {#if g.icon}<img src={mediaUrl(g.icon)} alt="" />{:else}<span>{initials(g.name)}</span>{/if}
        <span class="tip">{g.name}</span>
      </button>
    {/each}
    <hr />
    <button class="server add" aria-label="Criar ou entrar em um grupo" onclick={() => (modal = 'add')}>
      <Icon name="plus" size={24} />
      <span class="tip">Criar ou entrar em um grupo</span>
    </button>
  </nav>

  <aside class="sidebar">
    {#if guild}
      <header class="server-name">
        <button class="menu-btn" aria-expanded={menu} onclick={(e) => (e.stopPropagation(), (menu = !menu))}>
          <span>{guild.name}</span>
          <Icon name={menu ? 'close' : 'chevron'} size={18} />
        </button>
        {#if menu}
          <div class="menu" role="menu">
            <button role="menuitem" class="accent" onclick={invite}>Convidar pessoas <Icon name="invite" size={18} /></button>
            {#if owner}
              <button role="menuitem" onclick={() => ((modal = 'server'), (menu = false))}>Configurações do grupo <Icon name="gear" size={18} /></button>
            {:else}
              <button role="menuitem" class="danger" onclick={leave}>Sair do grupo <Icon name="leave" size={18} /></button>
            {/if}
          </div>
        {/if}
      </header>

      <div class="channels">
        {#each [{ kind: 'text', label: 'Canais de texto' }, { kind: 'voice', label: 'Canais de voz' }] as const as section}
          <div class="category">
            <span>{section.label}</span>
            {#if owner}<button title="Criar canal" onclick={() => toggleCreating(section.kind)}><Icon name="plus" size={16} /></button>{/if}
          </div>
          {#if creating === section.kind}
            <form class="new" onsubmit={createChannel}>
              <!-- svelte-ignore a11y_autofocus -->
              <input bind:value={newName} placeholder="novo-canal" maxlength="32" autofocus onkeydown={(e) => e.key === 'Escape' && toggleCreating(section.kind)} />
            </form>
          {/if}
          {#each channels.filter((c) => c.kind === section.kind) as c (c.id)}
            <div class="channel-row">
              <button
                class="channel"
                class:active={c.id === currentId}
                class:unread={unread.has(c.id)}
                onclick={() => open(c).catch(notify)}
                onmouseenter={() => c.kind === 'voice' && voice.prefetch(c.id).catch(() => {})}
              >
                <Icon name={c.kind === 'text' ? 'hash' : 'volume'} size={20} />
                <span>{c.name}</span>
              </button>
              {#if owner}<button class="edit" title="Editar canais" aria-label="Editar canais" onclick={() => (modal = 'server')}><Icon name="gear" size={14} /></button>{/if}
            </div>
            {#if c.kind === 'voice'}
              {#each members(c.id) as p (p.name)}
                <div role="group" onmouseenter={(e) => p.sharing && showPreview(e, p.name)} onmouseleave={() => (preview = null)}>
                  <button class="voice-member" class:speaking={voice.speaking.includes(p.name)} onclick={(e) => showCard(e, p.name)} oncontextmenu={(e) => showListen(e, p.name)}>
                    <Avatar name={p.name} size={24} />
                    <Name name={p.name} />
                    <VoiceStatus {p} />
                  </button>
                  {#if preview?.name === p.name && p.sharing}
                    <StreamPreview {...preview} onwatch={() => ((preview = null), (voice.focus = p.name), open(c).catch(notify))} />
                  {/if}
                </div>
              {/each}
            {/if}
          {/each}
        {/each}
      </div>
    {:else}
      <div class="no-server">
        <p>Você ainda não está em nenhum grupo.</p>
        <button class="btn" onclick={() => (modal = 'add')}>Criar ou entrar</button>
      </div>
    {/if}

    {#if voice.channel || voice.connecting}
      <div class="voice-panel">
        <div class="voice-status">
          <strong class:connecting={voice.connecting}>{voice.connecting ? 'Conectando…' : 'Voz conectada'}</strong>
          <small>{voiceWhere}</small>
        </div>
        <button class="icon-btn" class:on={voice.sharing} title={voice.sharing ? 'Parar transmissão' : 'Compartilhar tela'} onclick={() => voice.toggleScreen().catch(notify)}>
          <Icon name="screen" size={20} />
        </button>
        {#if guilds.find((g) => g.id === voiceServer)?.owner === me.id}
          <button class="icon-btn" title="Tiro (toca para a call toda)" onclick={shoot}><Icon name="target" size={20} /></button>
        {/if}
        <button class="icon-btn" title="Desconectar" onclick={() => voice.leave()}><Icon name="leave" size={20} /></button>
      </div>
    {/if}

    <footer class="me">
      <button class="who" onclick={(e) => showCard(e, me.name)}>
        <Avatar name={me.name} size={32} online ring="var(--panel)" />
        <span>
          <strong><Name name={me.name} /></strong>
          <small>Online</small>
        </span>
      </button>
      <button class="icon-btn" class:off={voice.muted} title={voice.muted ? 'Ativar microfone' : 'Silenciar'} onclick={() => voice.toggleMute().catch(notify)}>
        <Icon name={voice.muted ? 'micOff' : 'mic'} size={20} />
      </button>
      <button class="icon-btn" class:off={voice.deafened} title={voice.deafened ? 'Ativar áudio' : 'Desativar áudio'} onclick={() => voice.toggleDeafen().catch(notify)}>
        <Icon name={voice.deafened ? 'headphonesOff' : 'headphones'} size={20} />
      </button>
      <button class="icon-btn" title="Configurações" onclick={() => (modal = 'settings')}><Icon name="gear" size={20} /></button>
    </footer>
  </aside>

  {#if current?.kind === 'voice'}
    <Stage channel={current} members={members(current.id)} onerror={notify} onmenu={showListen} onjoin={() => open(current).catch(notify)} />
  {:else if current}
    <main class="chat" ondragover={(e) => e.preventDefault()} ondrop={(e) => (e.preventDefault(), attach(e.dataTransfer?.files[0]))}>
      <header class="topbar">
        <Icon name="hash" size={22} />
        <h1>{current.name}</h1>
        <button class="icon-btn members-toggle" class:on={showMembers} title={showMembers ? 'Ocultar membros' : 'Mostrar membros'} onclick={() => (showMembers = !showMembers)}>
          <Icon name="users" size={22} />
        </button>
      </header>

      <div class="messages" bind:this={scroller} onscroll={() => older().catch(notify)} onloadcapture={keepBottom} onloadedmetadatacapture={keepBottom}>
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
              {#if m.body}<p>{m.body}</p>{/if}
              {#if m.file}<Attachment file={m.file} onview={(url) => (viewing = url)} />{/if}
            </div>
          {:else}
            <div class="msg head">
              <button class="pic" aria-label="Perfil de {m.author}" onclick={(e) => showCard(e, m.author)}><Avatar name={m.author} /></button>
              <div>
                <div class="meta">
                  <button class="author" onclick={(e) => showCard(e, m.author)}><Name name={m.author} /></button>
                  <time>{when(m.createdAt)}</time>
                </div>
                {#if m.body}<p>{m.body}</p>{/if}
                {#if m.file}<Attachment file={m.file} onview={(url) => (viewing = url)} />{/if}
              </div>
            </div>
          {/if}
        {/each}
        <div class="end"></div>
      </div>

      <form class="composer" onsubmit={(e) => (e.preventDefault(), send())}>
        {#if pending}
          <div class="pending">
            {#if pending.file.type.startsWith('video/')}
              <!-- svelte-ignore a11y_media_has_caption -->
              <video src={pending.preview} muted></video>
            {:else}
              <img src={pending.preview} alt="" />
            {/if}
            <span>{sending ? 'Enviando…' : pending.file.name}</span>
            <button type="button" class="icon-btn" title="Remover anexo" aria-label="Remover anexo" disabled={sending} onclick={unattach}><Icon name="trash" size={18} /></button>
          </div>
        {/if}
        <div class="compose-row">
          <label class="attach" title="Enviar foto ou vídeo">
            <Icon name="plusCircle" size={22} />
            <input type="file" accept="image/png,image/jpeg,image/gif,image/webp,video/mp4,video/webm" hidden onchange={(e) => (attach(e.currentTarget.files?.[0]), (e.currentTarget.value = ''))} />
          </label>
          <textarea
            bind:value={draft}
            onkeydown={onKey}
            onpaste={(e) => {
              const file = e.clipboardData?.files[0]
              if (file) (e.preventDefault(), attach(file))
            }}
            rows="1"
            maxlength="4000"
            placeholder="Conversar em #{current.name}"
          ></textarea>
        </div>
      </form>
    </main>

    {#if showMembers}
      <aside class="members">
        {#each [{ label: 'Online', list: roster.filter((n) => online.has(n)) }, { label: 'Offline', list: roster.filter((n) => !online.has(n)) }] as group (group.label)}
          {#if group.list.length}
            <h3>{group.label} · {group.list.length}</h3>
            {#each group.list as name (name)}
              <button class="member" class:away={group.label === 'Offline'} onclick={(e) => showCard(e, name)}>
                <Avatar {name} size={32} online={online.has(name)} />
                <span class="name"><Name {name} /></span>
                {#if presence.find((p) => p.name === name)?.voice}<span class="in-voice" title="Em canal de voz"><Icon name="volume" size={16} /></span>{/if}
                {#if guild?.owner === people[name]?.id}<span class="crown" title="Dono do grupo"><Icon name="crown" size={14} /></span>{/if}
              </button>
            {/each}
          {/if}
        {/each}
      </aside>
    {/if}
  {:else}
    <main class="chat empty-chat">
      <p>{guild ? 'Nenhum canal de texto por aqui.' : ''}</p>
    </main>
  {/if}

  {#if card && people[card.name]}
    <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
    <div class="popout" style:left="{card.x}px" style:top="{card.y}px" onclick={(e) => e.stopPropagation()}>
      <ProfileCard profile={people[card.name]} online={online.has(card.name)} />
    </div>
  {/if}

  {#if listenMenu}<VoiceUserMenu {...listenMenu} />{/if}
  {#if viewing}
    <div class="backdrop viewer" role="presentation" onclick={() => (viewing = '')}>
      <img src={viewing} alt="" />
    </div>
  {/if}
  {#if toast}<div class="toast" role="status">{toast}</div>{/if}
  {#if modal === 'settings'}<Settings onclose={() => (modal = null)} onerror={notify} />{/if}
  {#if modal === 'server' && guild}<ServerSettings {guild} {channels} onclose={() => (modal = null)} onerror={notify} />{/if}
  {#if modal === 'add'}<AddServer onclose={() => (modal = null)} ondone={(id) => ((modal = null), serverChanged(id).then(() => openServer(id)).catch(notify))} />{/if}
  {#if voice.picking}<SourcePicker onerror={notify} />{/if}
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
    font-weight: 700;
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
    width: 100%;
    border-radius: 4px;
    text-align: left;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 8px 4px 34px;
    font-size: 14px;
    color: var(--muted);
  }

  .voice-member :global(.avatar) {
    transition: box-shadow 0.15s;
  }

  .voice-member:hover {
    background: var(--hover);
  }

  .voice-member.speaking :global(.avatar) {
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
    align-items: center;
    gap: 8px;
    min-width: 0;
    padding: 4px;
    margin-right: 4px;
    border-radius: 4px;
    text-align: left;
    line-height: 1.2;
  }

  .who:hover {
    background: var(--hover);
  }

  .who > span {
    display: flex;
    flex-direction: column;
    min-width: 0;
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
    background: color-mix(in srgb, var(--chat), #000 6%);
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

  .head .pic {
    margin-top: 2px;
  }

  .meta {
    display: flex;
    align-items: baseline;
    gap: 8px;
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

  .compose-row {
    display: flex;
    align-items: flex-start;
    gap: 12px;
  }

  .attach {
    display: grid;
    margin-top: 9px;
    color: var(--muted);
    cursor: pointer;
  }

  .attach:hover {
    color: var(--strong);
  }

  .pending {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 0;
    border-bottom: 1px solid var(--line);
  }

  .pending img,
  .pending video {
    width: 96px;
    height: 72px;
    object-fit: cover;
    border-radius: 4px;
    background: var(--rail);
  }

  .pending span {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 14px;
  }

  .viewer {
    cursor: zoom-out;
  }

  .viewer img {
    max-width: 90vw;
    max-height: 90vh;
    border-radius: 4px;
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
    background: var(--rail);
    box-shadow: 0 8px 24px #0008;
  }

  .rail {
    gap: 8px;
    overflow-y: auto;
    padding-bottom: 12px;
  }

  .rail hr {
    width: 32px;
    margin: 0;
    border: 0;
    border-top: 2px solid var(--line);
  }

  .server {
    border-radius: 50%;
    background: var(--sidebar);
    color: var(--strong);
    font-size: 16px;
    transition:
      border-radius 0.2s,
      background 0.2s;
  }

  .server img {
    width: 100%;
    height: 100%;
    border-radius: inherit;
    object-fit: cover;
  }

  .server:hover,
  .server.active {
    border-radius: 16px;
    background: var(--accent);
    color: #fff;
  }

  .server::before {
    content: '';
    position: absolute;
    left: -12px;
    width: 4px;
    height: 0;
    border-radius: 0 4px 4px 0;
    background: var(--strong);
    transition: height 0.2s;
  }

  .server.unread::before {
    height: 8px;
  }

  .server:hover::before {
    height: 20px;
  }

  .server.active::before {
    height: 40px;
  }

  .server.add {
    color: var(--green);
  }

  .server.add:hover {
    background: var(--green);
    color: #fff;
  }

  .tip {
    position: fixed;
    left: 80px;
    z-index: 30;
    padding: 6px 10px;
    border-radius: 6px;
    font-size: 14px;
    font-weight: 600;
    white-space: nowrap;
    color: var(--strong);
    background: var(--rail);
    box-shadow: 0 4px 16px #0006;
    pointer-events: none;
    opacity: 0;
    transform: translateX(-4px);
    transition:
      opacity 0.1s,
      transform 0.1s;
  }

  .server:hover .tip {
    opacity: 1;
    transform: none;
  }

  .server-name {
    position: relative;
    padding: 0;
  }

  .menu-btn {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    height: 100%;
    padding: 0 12px 0 16px;
    font-weight: 600;
    color: var(--strong);
  }

  .menu-btn span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .menu-btn:hover {
    background: var(--hover);
  }

  .menu {
    position: absolute;
    top: 52px;
    left: 10px;
    right: 10px;
    z-index: 5;
    padding: 6px 8px;
    border-radius: var(--radius);
    background: var(--rail);
    box-shadow: 0 8px 24px #0007;
    animation: pop 0.12s ease-out;
  }

  .menu button {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    padding: 8px;
    border-radius: 4px;
    font-size: 14px;
    font-weight: 500;
    color: var(--text);
  }

  .menu button:hover {
    background: var(--accent);
    color: #fff;
  }

  .menu .accent {
    color: color-mix(in srgb, var(--accent), #fff 35%);
  }

  .menu .danger {
    color: var(--red);
  }

  .menu .danger:hover {
    background: var(--red);
  }

  .channel-row {
    position: relative;
  }

  .edit {
    position: absolute;
    right: 6px;
    top: 50%;
    display: grid;
    padding: 3px;
    color: var(--muted);
    opacity: 0;
    transform: translateY(-50%);
  }

  .channel-row:hover .edit,
  .edit:focus-visible {
    opacity: 1;
  }

  .edit:hover {
    color: var(--strong);
  }

  .no-server {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    padding: 16px;
    text-align: center;
    color: var(--muted);
  }

  .pic,
  .author {
    display: block;
    height: max-content;
    border-radius: 50%;
  }

  .author {
    border-radius: 0;
    font-weight: 500;
    color: var(--strong);
  }

  .author:hover {
    text-decoration: underline;
  }

  .member {
    width: 100%;
    text-align: left;
  }

  .member.away {
    opacity: 0.45;
  }

  .member.away:hover {
    opacity: 1;
  }

  .crown {
    display: grid;
    color: #f0b232;
  }

  .popout {
    position: fixed;
    z-index: 15;
    animation: pop 0.15s ease-out;
  }

  .empty-chat {
    display: grid;
    place-items: center;
    color: var(--muted);
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
