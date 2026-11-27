<script lang="ts">
  import { onMount } from 'svelte'
  import { Quit, WindowIsMaximised, WindowMinimise, WindowToggleMaximise } from '../wailsjs/runtime/runtime'

  let maximised = $state(false)

  onMount(() => {
    const sync = () => WindowIsMaximised().then((m) => (maximised = m))
    sync()
    window.addEventListener('resize', sync)
    return () => window.removeEventListener('resize', sync)
  })
</script>

<!-- O Wails arrasta a janela por --wails-draggable, mas não maximiza no duplo clique.
     O duplo clique só repete o botão Maximizar, que segue acessível pelo teclado. -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<header class="titlebar" ondblclick={(e) => !(e.target as Element).closest('button') && WindowToggleMaximise()}>
  <div class="title">
    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
      <rect width="24" height="24" rx="5.3" fill="#5865f2" />
      <path d="M16.1 8.2A6 6 0 1 0 16.1 15.8" fill="none" stroke="#fff" stroke-width="2.6" stroke-linecap="round" />
    </svg>
    Concord
  </div>
  <div class="controls">
    <button title="Minimizar" aria-label="Minimizar" onclick={WindowMinimise}>
      <svg viewBox="0 0 10 10" width="10" height="10"><path d="M0 5h10" stroke="currentColor" /></svg>
    </button>
    <button title={maximised ? 'Restaurar' : 'Maximizar'} aria-label={maximised ? 'Restaurar' : 'Maximizar'} onclick={WindowToggleMaximise}>
      {#if maximised}
        <svg viewBox="0 0 10 10" width="10" height="10" fill="none" stroke="currentColor"><path d="M2.5 2.5V.5h7v7h-2M.5 2.5h7v7h-7z" /></svg>
      {:else}
        <svg viewBox="0 0 10 10" width="10" height="10" fill="none" stroke="currentColor"><rect x=".5" y=".5" width="9" height="9" /></svg>
      {/if}
    </button>
    <button class="close" title="Fechar" aria-label="Fechar" onclick={Quit}>
      <svg viewBox="0 0 10 10" width="10" height="10"><path d="M0 0l10 10M10 0 0 10" stroke="currentColor" stroke-width="1.1" /></svg>
    </button>
  </div>
</header>

<style>
  .titlebar {
    --wails-draggable: drag;
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
    --wails-draggable: no-drag;
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
