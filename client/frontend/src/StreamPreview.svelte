<script lang="ts">
  import type { Track } from 'livekit-client'
  import { voice } from './lib/voice.svelte'
  import Avatar from './lib/Avatar.svelte'
  import Icon from './lib/Icon.svelte'

  let { name, x, y, onwatch }: { name: string; x: number; y: number; onwatch: () => void } = $props()

  // Só dá para ver a imagem ao vivo de quem está na mesma call que eu.
  const screen = $derived(voice.screens.find((s) => s.id === name))

  function attach(el: HTMLVideoElement, track: Track) {
    track.attach(el)
    return { destroy: () => track.detach(el) }
  }
</script>

<button class="preview" style:left="{x}px" style:top="{Math.min(y, innerHeight - 190)}px" onclick={onwatch}>
  <span class="frame">
    {#if screen}
      <video use:attach={screen.track} autoplay playsinline muted></video>
    {:else}
      <Avatar {name} size={56} />
    {/if}
  </span>
  <span class="cta"><Icon name="screen" size={16} /> Assistir a transmissão</span>
</button>

<style>
  /* O padding à esquerda cobre o vão até a lista, senão o hover cai no caminho. */
  .preview {
    position: fixed;
    z-index: 15;
    display: flex;
    flex-direction: column;
    width: 268px;
    padding: 0 0 0 12px;
    animation: pop 0.12s ease-out;
  }

  .frame {
    display: grid;
    place-items: center;
    height: 144px;
    overflow: hidden;
    border-radius: var(--radius) var(--radius) 0 0;
    background: #000;
  }

  video {
    width: 100%;
    height: 100%;
    object-fit: contain;
  }

  .cta {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 8px;
    border-radius: 0 0 var(--radius) var(--radius);
    font-size: 14px;
    font-weight: 600;
    color: #fff;
    background: var(--accent);
    box-shadow: 0 8px 24px #0007;
  }

  .preview:hover .cta {
    background: var(--accent-hover);
  }
</style>
