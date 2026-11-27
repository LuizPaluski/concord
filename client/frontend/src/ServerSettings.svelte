<script lang="ts">
  import { api, mediaUrl, uploadImage, type Channel, type Guild } from './lib/api.svelte'
  import Icon from './lib/Icon.svelte'

  let { guild, channels, onclose, onerror }: { guild: Guild; channels: Channel[]; onclose: () => void; onerror: (err: unknown) => void } = $props()

  // Rascunho do formulário: começa com o valor salvo e não acompanha mudanças de fora.
  // svelte-ignore state_referenced_locally
  let name = $state(guild.name)
  // svelte-ignore state_referenced_locally
  let icon = $state(guild.icon)
  let names = $state<Record<number, string>>({})
  let confirming = $state(false)
  let busy = $state(false)

  async function run(fn: () => Promise<unknown>) {
    busy = true
    try {
      await fn()
    } catch (err) {
      onerror(err)
    } finally {
      busy = false
    }
  }

  async function pickIcon(e: Event) {
    const input = e.currentTarget as HTMLInputElement
    const file = input.files?.[0]
    input.value = ''
    if (file) await run(async () => (icon = await uploadImage(file, 256, 256)))
  }

  const saveServer = () => run(() => api(`/api/servers/${guild.id}`, { method: 'POST', body: { name, icon } }))

  function rename(c: Channel) {
    const next = names[c.id]?.trim()
    if (!next || next === c.name) return
    run(() => api(`/api/channels/${c.id}`, { method: 'POST', body: { name: next } }))
  }

  function remove(c: Channel) {
    if (confirm(`Apagar ${c.kind === 'text' ? '#' : ''}${c.name}? As mensagens dele somem junto.`)) run(() => api(`/api/channels/${c.id}`, { method: 'DELETE' }))
  }

  const removeServer = () => run(async () => (await api(`/api/servers/${guild.id}`, { method: 'DELETE' }), onclose()))
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="backdrop" role="presentation" onclick={(e) => e.target === e.currentTarget && onclose()}>
  <div class="modal wide" role="dialog" aria-modal="true" aria-labelledby="server-title">
    <header>
      <h2 id="server-title">Configurações do grupo</h2>
      <button class="icon-btn" title="Fechar" aria-label="Fechar" onclick={onclose}><Icon name="close" /></button>
    </header>

    <div class="overview">
      <label class="pic" title="Trocar ícone">
        {#if icon}<img src={mediaUrl(icon)} alt="" />{:else}<span>{name[0] ?? '?'}</span>{/if}
        <em>Trocar</em>
        <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" hidden onchange={pickIcon} />
      </label>
      <div class="grow">
        <span class="label">Nome do grupo</span>
        <input bind:value={name} maxlength="32" />
        <div class="row-actions">
          {#if icon}<button class="btn ghost" onclick={() => (icon = '')}>Remover ícone</button>{/if}
          <button class="btn" disabled={busy || !name.trim() || (name === guild.name && icon === guild.icon)} onclick={saveServer}>Salvar</button>
        </div>
      </div>
    </div>

    <span class="label">Canais</span>
    <ul class="channels">
      {#each channels as c (c.id)}
        <li>
          <Icon name={c.kind === 'text' ? 'hash' : 'volume'} size={18} />
          <input
            value={names[c.id] ?? c.name}
            maxlength="32"
            aria-label="Nome do canal {c.name}"
            oninput={(e) => (names[c.id] = e.currentTarget.value)}
            onblur={() => rename(c)}
            onkeydown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
          />
          <button class="icon-btn" title="Apagar canal" aria-label="Apagar canal {c.name}" disabled={busy} onclick={() => remove(c)}><Icon name="trash" size={18} /></button>
        </li>
      {/each}
    </ul>
    <small class="hint">Clique no nome para renomear. Para criar canal, use o + ao lado da categoria.</small>

    <div class="danger-zone">
      {#if confirming}
        <p>Apagar <strong>{guild.name}</strong> remove todos os canais e mensagens. Não tem volta.</p>
        <div class="row-actions">
          <button class="btn ghost" onclick={() => (confirming = false)}>Cancelar</button>
          <button class="btn danger" disabled={busy} onclick={removeServer}>Apagar grupo</button>
        </div>
      {:else}
        <button class="btn danger" onclick={() => (confirming = true)}>Apagar grupo</button>
      {/if}
    </div>
  </div>
</div>

<style>
  .wide {
    width: min(560px, calc(100% - 32px));
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
  }

  header h2 {
    margin: 0;
    text-align: left;
  }

  .overview {
    display: flex;
    gap: 20px;
    align-items: flex-start;
  }

  .pic {
    position: relative;
    flex: none;
    display: grid;
    place-items: center;
    width: 96px;
    height: 96px;
    margin-top: 16px;
    overflow: hidden;
    border-radius: 50%;
    background: var(--accent);
    color: #fff;
    font-size: 36px;
    font-weight: 700;
    cursor: pointer;
  }

  .pic img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .pic em {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    font-size: 12px;
    font-style: normal;
    font-weight: 700;
    text-transform: uppercase;
    background: #0009;
    opacity: 0;
    transition: opacity 0.15s;
  }

  .pic:hover em {
    opacity: 1;
  }

  .grow {
    flex: 1;
  }

  .row-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }

  .channels {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .channels li {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 2px 4px 2px 10px;
    border-radius: 4px;
    color: var(--muted);
  }

  .channels li:hover {
    background: var(--hover);
  }

  .channels input {
    padding: 6px;
    background: none;
  }

  .channels input:focus {
    background: var(--rail);
  }

  .hint {
    display: block;
    margin-top: 8px;
    font-size: 13px;
    color: var(--faint);
  }

  .danger-zone {
    margin-top: 24px;
    padding: 16px;
    border: 1px solid color-mix(in srgb, var(--red), transparent 50%);
    border-radius: var(--radius);
  }

  .danger-zone p {
    margin: 0;
  }
</style>
