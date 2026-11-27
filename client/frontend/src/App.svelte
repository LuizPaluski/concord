<script lang="ts">
  import { auth } from './lib/api.svelte'
  import Chat from './Chat.svelte'
  import Login from './Login.svelte'
  import Updater from './Updater.svelte'
</script>

{#if auth.session}
  <Chat />
{:else}
  <Login />
{/if}
<Updater />
