<script lang="ts">
  import { auth } from './lib/api.svelte'
  import Chat from './Chat.svelte'
  import Login from './Login.svelte'
  import TitleBar from './TitleBar.svelte'
  import Updater from './Updater.svelte'

  // No navegador (testes, vite dev) não existe janela do Wails para controlar.
  const desktop = 'go' in window
</script>

<div class="app" class:desktop>
  {#if desktop}<TitleBar />{/if}
  <div class="view">
    {#if auth.session}
      <Chat />
    {:else}
      <Login />
    {/if}
  </div>
</div>
<Updater />

<style>
  .app {
    height: 100%;
    display: grid;
    grid-template-rows: 1fr;
  }

  .app.desktop {
    grid-template-rows: 24px 1fr;
  }

  .view {
    min-height: 0;
  }
</style>
