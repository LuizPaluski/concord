<script lang="ts">
  import { api, type Guild } from './lib/api.svelte'

  let { onclose, ondone }: { onclose: () => void; ondone: (id: number) => void } = $props()

  let mode = $state<'create' | 'join'>('create')
  let value = $state('')
  let error = $state('')
  let busy = $state(false)

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    error = ''
    try {
      const { id } =
        mode === 'create'
          ? await api<Guild>('/api/servers', { method: 'POST', body: { name: value } })
          : await api<{ id: number }>('/api/join', { method: 'POST', body: { invite: value } })
      ondone(id)
    } catch (err) {
      error = (err as Error).message
    } finally {
      busy = false
    }
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="backdrop" role="presentation" onclick={(e) => e.target === e.currentTarget && onclose()}>
  <form class="modal" aria-labelledby="add-title" onsubmit={submit}>
    <h2 id="add-title">{mode === 'create' ? 'Crie seu grupo' : 'Entre em um grupo'}</h2>
    <p class="lead">
      {mode === 'create' ? 'Um espaço novo com canais próprios. Depois é só chamar a galera com um convite.' : 'Cole o convite que alguém do grupo te mandou.'}
    </p>
    <label>
      <span class="label">{mode === 'create' ? 'Nome do grupo' : 'Convite'}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input bind:value maxlength={mode === 'create' ? 32 : 64} required autofocus autocomplete="off" />
    </label>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <footer>
      <button type="button" class="btn ghost" onclick={() => ((mode = mode === 'create' ? 'join' : 'create'), (value = error = ''))}>
        {mode === 'create' ? 'Tenho um convite' : 'Quero criar um'}
      </button>
      <button class="btn" disabled={busy}>{mode === 'create' ? 'Criar' : 'Entrar'}</button>
    </footer>
  </form>
</div>

<style>
  .error {
    margin: 8px 0 0;
    font-size: 14px;
    color: var(--red);
  }

  footer {
    display: flex;
    justify-content: space-between;
    margin-top: 24px;
  }
</style>
