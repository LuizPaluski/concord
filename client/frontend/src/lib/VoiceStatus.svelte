<script lang="ts">
  import type { Presence } from './api.svelte'
  import { settings } from './settings.svelte'
  import Icon from './Icon.svelte'

  let { p }: { p: Presence } = $props()

  const mine = $derived(settings.listen[p.name])
</script>

<span class="status">
  {#if p.sharing}<span class="live" title="Transmitindo a tela">Ao vivo</span>{/if}
  {#if mine?.muted}<span title="Você silenciou"><Icon name="volumeOff" size={15} /></span>{/if}
  {#if p.deafened}
    <span class="off" title="Áudio desativado"><Icon name="headphonesOff" size={15} /></span>
  {:else if p.muted}
    <span class="off" title="Microfone desativado"><Icon name="micOff" size={15} /></span>
  {/if}
</span>

<style>
  .status {
    display: flex;
    align-items: center;
    gap: 4px;
    margin-left: auto;
    color: var(--muted);
  }

  .status > span {
    display: grid;
  }

  .off {
    color: var(--red);
  }

  .live {
    padding: 1px 5px;
    border-radius: 4px;
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    color: #fff;
    background: var(--red);
  }
</style>
