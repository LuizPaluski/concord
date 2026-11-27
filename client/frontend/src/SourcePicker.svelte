<script lang="ts">
  import { onMount } from 'svelte'
  import { desktop, type ScreenSource } from './lib/desktop'
  import { settings } from './lib/settings.svelte'
  import { voice } from './lib/voice.svelte'
  import Icon from './lib/Icon.svelte'

  let { onerror }: { onerror: (err: unknown) => void } = $props()

  let sources = $state<ScreenSource[]>([])
  let tab = $state<'screen' | 'window'>('screen')
  let chosen = $state('')
  const shown = $derived(sources.filter((s) => s.screen === (tab === 'screen')))

  onMount(() => {
    desktop!.screenSources().then((list) => {
      sources = list
      chosen = list.find((s) => s.screen)?.id ?? ''
    }, onerror)
  })

  const close = () => (voice.picking = false)

  function share(id = chosen) {
    if (id) voice.shareSource(id).catch(onerror)
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && close()} />

<div class="backdrop" role="presentation" onclick={(e) => e.target === e.currentTarget && close()}>
  <div class="modal" role="dialog" aria-modal="true" aria-labelledby="picker-title">
    <header>
      <h2 id="picker-title">Compartilhar a tela</h2>
      <button class="icon-btn" title="Fechar" onclick={close}><Icon name="close" /></button>
    </header>

    <div class="tabs" role="group" aria-label="Tipo de fonte">
      <button aria-pressed={tab === 'screen'} class:on={tab === 'screen'} onclick={() => (tab = 'screen')}>Telas</button>
      <button aria-pressed={tab === 'window'} class:on={tab === 'window'} onclick={() => (tab = 'window')}>Janelas</button>
    </div>

    <div class="grid">
      {#each shown as s (s.id)}
        <button class="source" class:chosen={chosen === s.id} aria-pressed={chosen === s.id} onclick={() => (chosen = s.id)} ondblclick={() => share(s.id)}>
          <img src={s.thumbnail} alt="" />
          <span class="name">
            {#if s.icon}<img src={s.icon} alt="" width="16" height="16" />{/if}
            <span>{s.name}</span>
          </span>
        </button>
      {:else}
        <p class="empty">{sources.length ? 'Nada para mostrar aqui.' : 'Procurando telas e janelas…'}</p>
      {/each}
    </div>

    <footer>
      <span class="quality">{settings.screenRes}p · {settings.screenFps} fps{settings.screenAudio ? ' · com som' : ''}</span>
      <button class="cancel" onclick={close}>Cancelar</button>
      <button class="btn" disabled={!chosen} onclick={() => share()}>Transmitir</button>
    </footer>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 10;
    display: grid;
    place-items: center;
    background: #000a;
  }

  .modal {
    display: flex;
    flex-direction: column;
    width: min(760px, calc(100% - 32px));
    max-height: calc(100% - 64px);
    border-radius: var(--radius);
    background: var(--chat);
    box-shadow: 0 8px 40px #0008;
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 20px 24px 12px;
  }

  h2 {
    margin: 0;
    font-size: 20px;
    color: var(--strong);
  }

  .tabs {
    display: flex;
    gap: 4px;
    margin: 0 24px 12px;
  }

  .tabs button {
    padding: 6px 14px;
    border-radius: 4px;
    font-weight: 500;
    color: var(--muted);
  }

  .tabs button:hover {
    background: var(--hover);
    color: var(--text);
  }

  .tabs button.on {
    background: var(--active);
    color: var(--strong);
  }

  /* Sem min-height: 0 a grade cresce com muitas janelas e empurra o rodapé para fora. */
  .grid {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 12px;
    padding: 4px 24px 16px;
    overflow-y: auto;
  }

  .source {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 8px;
    border-radius: var(--radius);
    background: var(--sidebar);
    outline: 2px solid transparent;
    outline-offset: -2px;
    text-align: left;
  }

  .source:hover {
    background: var(--hover);
  }

  .source.chosen {
    outline-color: var(--accent);
  }

  .source > img {
    width: 100%;
    aspect-ratio: 16 / 9;
    object-fit: contain;
    border-radius: 4px;
    background: #000;
  }

  .name {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    font-size: 13px;
    color: var(--text);
  }

  .name span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .empty {
    grid-column: 1 / -1;
    margin: 24px 0;
    text-align: center;
    color: var(--muted);
  }

  footer {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 16px 24px;
    border-radius: 0 0 var(--radius) var(--radius);
    background: var(--sidebar);
  }

  .quality {
    flex: 1;
    font-size: 13px;
    color: var(--muted);
  }

  .cancel {
    padding: 0 12px;
    color: var(--text);
  }

  .cancel:hover {
    text-decoration: underline;
  }
</style>
