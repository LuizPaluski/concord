<script lang="ts">
  import { onMount } from 'svelte'
  import { desktop } from './lib/desktop'

  const win = desktop!
  let maximised = $state(false)

  onMount(() => {
    win.isMaximized().then((m) => (maximised = m))
    win.onMaximized((m) => (maximised = m))
  })
</script>

<header class="titlebar">
  <div class="title">
    <img src="/icon.png" width="16" height="16" alt="" />
    Concord
  </div>
  <div class="controls">
    <button title="Minimizar" aria-label="Minimizar" onclick={win.minimize}>
      <svg viewBox="0 0 10 10" width="10" height="10"><path d="M0 5h10" stroke="currentColor" /></svg>
    </button>
    <button title={maximised ? 'Restaurar' : 'Maximizar'} aria-label={maximised ? 'Restaurar' : 'Maximizar'} onclick={win.toggleMaximize}>
      {#if maximised}
        <svg viewBox="0 0 10 10" width="10" height="10" fill="none" stroke="currentColor"><path d="M2.5 2.5V.5h7v7h-2M.5 2.5h7v7h-7z" /></svg>
      {:else}
        <svg viewBox="0 0 10 10" width="10" height="10" fill="none" stroke="currentColor"><rect x=".5" y=".5" width="9" height="9" /></svg>
      {/if}
    </button>
    <button class="close" title="Fechar" aria-label="Fechar" onclick={win.close}>
      <svg viewBox="0 0 10 10" width="10" height="10"><path d="M0 0l10 10M10 0 0 10" stroke="currentColor" stroke-width="1.1" /></svg>
    </button>
  </div>
</header>

<style>
  /* A área de arraste nativa já move a janela e maximiza no duplo clique. */
  .titlebar {
    -webkit-app-region: drag;
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    height: 24px;
    background: var(--rail);
    user-select: none;
  }

  .title {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    font-weight: 600;
    color: var(--muted);
  }

  .controls {
    -webkit-app-region: no-drag;
    position: absolute;
    top: 0;
    right: 0;
    display: flex;
    height: 100%;
  }

  .controls button {
    display: grid;
    place-items: center;
    width: 46px;
    height: 100%;
    color: var(--muted);
  }

  .controls button:hover {
    background: var(--hover);
    color: var(--strong);
  }

  .controls .close:hover {
    background: var(--red);
    color: #fff;
  }
</style>
