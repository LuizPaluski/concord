<script lang="ts">
  import type { Track } from 'livekit-client'
  import { hue, type Channel, type Presence } from './lib/api.svelte'
  import { voice } from './lib/voice.svelte'
  import Icon from './lib/Icon.svelte'

  let { channel, members, onerror }: { channel: Channel; members: Presence[]; onerror: (err: unknown) => void } = $props()

  const joined = $derived(voice.channel === channel.id)

  function attach(el: HTMLVideoElement, track: Track) {
    track.attach(el)
    return { destroy: () => track.detach(el) }
  }
</script>

<main class="stage">
  <header class="topbar"><Icon name="volume" /> {channel.name}</header>

  <div class="content">
    {#if joined && voice.screens.length}
      <div class="screens" class:single={voice.screens.length === 1}>
        {#each voice.screens as s (s.track.sid)}
          <figure>
            <video use:attach={s.track} autoplay playsinline muted ondblclick={(e) => e.currentTarget.requestFullscreen()}></video>
            <figcaption>{s.name}</figcaption>
          </figure>
        {/each}
      </div>
    {/if}

    <div class="people">
      {#each members as p (p.name)}
        <div class="tile" class:speaking={voice.speaking.includes(p.name)}>
          <span class="avatar" style:--h={hue(p.name)}>{p.name[0]}</span>
          <span>{p.name}</span>
        </div>
      {:else}
        <p class="empty">Ninguém por aqui ainda.</p>
      {/each}
    </div>

    {#if !joined}
      <button class="join" disabled={voice.connecting} onclick={() => voice.join(channel.id).catch(onerror)}>
        {voice.connecting ? 'Conectando…' : 'Entrar na sala'}
      </button>
    {/if}
  </div>
</main>

<style>
  .stage {
    grid-column: 2 / -1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    background: #090a0d;
  }

  .topbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 14px 20px;
    font-weight: 600;
    border-bottom: 1px solid var(--line);
  }

  .content {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 20px;
    padding: 20px;
  }

  .screens {
    width: 100%;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(420px, 1fr));
    gap: 12px;
  }

  .screens.single {
    grid-template-columns: 1fr;
  }

  figure {
    position: relative;
    margin: 0;
    border-radius: var(--radius);
    overflow: hidden;
    background: #000;
  }

  video {
    display: block;
    width: 100%;
    max-height: 70vh;
    object-fit: contain;
  }

  figcaption {
    position: absolute;
    left: 10px;
    bottom: 10px;
    padding: 3px 8px;
    border-radius: 6px;
    font-size: 13px;
    background: #000a;
  }

  .people {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 12px;
  }

  .tile {
    width: 160px;
    height: 120px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    border-radius: var(--radius);
    background: var(--panel-2);
    border: 2px solid transparent;
    transition: border-color 0.15s;
  }

  .tile.speaking {
    border-color: var(--speak);
  }

  .tile .avatar {
    width: 52px;
    height: 52px;
    font-size: 20px;
  }

  .empty {
    color: var(--muted);
  }

  .join {
    padding: 11px 22px;
    border-radius: 999px;
    font-weight: 600;
    background: var(--accent);
    color: var(--accent-ink);
  }

  .join:disabled {
    opacity: 0.6;
  }
</style>
