<script lang="ts">
  import { auth } from './lib/api.svelte'
  import { spotify } from './lib/spotify.svelte'
  import Name from './lib/Name.svelte'

  let busy = $state(false)

  async function run(fn: () => unknown) {
    busy = true
    spotify.error = ''
    try {
      await fn()
    } catch (err) {
      spotify.error = (err as Error).message
    } finally {
      busy = false
    }
  }

  const mine = $derived(spotify.jam?.host === auth.session?.user.name)
</script>

<div class="spotify">
  {#if spotify.jam}
    <div class="track">
      {#if spotify.jam.state.image}<img src={spotify.jam.state.image} alt="" />{/if}
      <div class="info">
        <strong>{spotify.jam.state.name}</strong>
        <small>{spotify.jam.state.artists}</small>
        <small class="host">{spotify.jam.state.playing ? 'Tocando' : 'Pausado'} por <Name name={spotify.jam.host} /></small>
      </div>
    </div>
    {#if mine}
      <button class="pill" disabled={busy} onclick={() => spotify.stopHosting()}>Parar de compartilhar</button>
    {:else if !spotify.connected}
      <button class="pill go" disabled={busy} onclick={() => run(() => spotify.login())}>Conectar Spotify para ouvir junto</button>
    {:else if spotify.listening}
      <button class="pill" disabled={busy} onclick={() => run(() => spotify.stopListening())}>Parar de ouvir</button>
    {:else}
      <button class="pill go" disabled={busy} onclick={() => run(() => spotify.listen())}>Ouvir junto</button>
    {/if}
  {:else if spotify.connected}
    <button class="pill go" disabled={busy} onclick={() => spotify.startHosting()}>Compartilhar minha música</button>
  {:else}
    <button class="pill go" disabled={busy} onclick={() => run(() => spotify.login())}>Conectar Spotify</button>
  {/if}
  {#if spotify.error}<small class="fail">{spotify.error}</small>{/if}
</div>

<style>
  .spotify {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 10px 8px;
    background: var(--panel);
    border-bottom: 1px solid #ffffff0f;
  }

  .track {
    display: flex;
    gap: 10px;
    align-items: center;
    min-width: 0;
  }

  img {
    width: 44px;
    height: 44px;
    border-radius: 4px;
    flex: none;
  }

  .info {
    display: flex;
    flex-direction: column;
    min-width: 0;
    line-height: 1.25;
  }

  .info strong,
  .info small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .info strong {
    font-size: 13px;
    color: var(--strong);
  }

  .info small {
    font-size: 12px;
    color: var(--muted);
  }

  .host {
    color: #1ed760 !important;
  }

  .pill {
    padding: 6px 10px;
    border-radius: 999px;
    font-size: 13px;
    font-weight: 600;
    color: var(--text);
    background: var(--active);
  }

  .pill.go {
    color: #000;
    background: #1ed760;
  }

  .pill:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .pill:disabled {
    opacity: 0.6;
  }

  .fail {
    font-size: 12px;
    color: var(--red);
  }
</style>
