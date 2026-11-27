<script lang="ts">
  import { auth } from './lib/api.svelte'
  import Chat from './Chat.svelte'
  import Login from './Login.svelte'
</script>

{#if auth.session}
  <Chat />
{:else}
  <Login />
{/if}
