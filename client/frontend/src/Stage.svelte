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
  <header class="topbar">
    <Icon name="volume" size={22} />
    <h1>{channel.name}</h1>
  </header>

  <div class="content">
    {#if joined && voice.screens.length}
      <div class="screens" class:single={voice.screens.length === 1}>
        {#each voice.screens as s (s.track.sid)}
          <figure>
            <video use:attach={s.track} autoplay playsinline muted ondblclick={(e) => e.currentTarget.requestFullscreen()}></video>
            <figcaption><Icon name="screen" size={14} /> {s.name}</figcaption>
          </figure>
        {/each}
      </div>
    {/if}

    {#if members.length}
      <div class="tiles" class:compact={joined && voice.screens.length > 0}>
        {#each members as p (p.name)}
          <div class="tile" class:speaking={voice.speaking.includes(p.name)} style:--h={hue(p.name)}>
            <span class="avatar" style:--size={joined && voice.screens.length ? '44px' : '80px'}>{p.name[0]}</span>
            <span class="label">{p.name}</span>
          </div>
        {/each}
      </div>
    {:else}
      <div class="empty">
        <h2>{channel.name}</h2>
        <p>Ninguém por aqui ainda.</p>
      </div>
    {/if}
  </div>

  <footer class="controls">
    {#if joined}
      <button class="round" class:off={voice.muted} title={voice.muted ? 'Ativar microfone' : 'Silenciar'} onclick={() => voice.toggleMute().catch(onerror)}>
        <Icon name={voice.muted ? 'micOff' : 'mic'} size={22} />
      </button>
      <button class="round" class:on={voice.sharing} title={voice.sharing ? 'Parar transmissão' : 'Compartilhar tela'} onclick={() => voice.toggleScreen().catch(onerror)}>
        <Icon name="screen" size={22} />
      </button>
      <button class="round leave" title="Desconectar" onclick={() => voice.leave()}><Icon name="leave" size={22} /></button>
    {:else}
      <button class="btn join" disabled={voice.connecting} onclick={() => voice.join(channel.id).catch(onerror)}>
        {voice.connecting ? 'Conectando…' : 'Entrar na sala de voz'}
      </button>
    {/if}
  </footer>
</main>

<style>
  .stage {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    background: #000;
  }

  .topbar {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 48px;
    padding: 0 16px;
    color: var(--faint);
  }

  .topbar h1 {
    margin: 0;
    font-size: 16px;
    color: var(--strong);
  }

  .content {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 12px;
    padding: 8px 16px;
  }

  .screens {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(420px, 1fr));
    gap: 8px;
  }

  .screens.single {
    grid-template-columns: 1fr;
  }

  figure {
    position: relative;
    margin: 0;
    border-radius: var(--radius);
    overflow: hidden;
    background: #111214;
  }

  video {
    display: block;
    width: 100%;
    max-height: 68vh;
    object-fit: contain;
  }

  figcaption {
    position: absolute;
    left: 8px;
    bottom: 8px;
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 8px;
    border-radius: 4px;
    font-size: 13px;
    font-weight: 500;
    color: var(--strong);
    background: #000b;
  }

  .tiles {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 320px));
    justify-content: center;
    gap: 8px;
  }

  .tiles.compact {
    grid-template-columns: repeat(auto-fit, 160px);
  }

  .tile {
    position: relative;
    display: grid;
    place-items: center;
    aspect-ratio: 16 / 9;
    border-radius: var(--radius);
    background: hsl(var(--h) 30% 22%);
    outline: 2px solid transparent;
    outline-offset: -2px;
    transition: outline-color 0.15s;
  }

  .tile.speaking {
    outline-color: var(--green);
  }

  .label {
    position: absolute;
    left: 8px;
    bottom: 8px;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 13px;
    font-weight: 500;
    color: var(--strong);
    background: #0009;
  }

  .empty {
    text-align: center;
  }

  .empty h2 {
    margin: 0 0 4px;
    font-size: 24px;
    color: var(--strong);
  }

  .empty p {
    margin: 0;
    color: var(--muted);
  }

  .controls {
    display: flex;
    justify-content: center;
    gap: 12px;
    padding: 16px 0 24px;
  }

  .round {
    display: grid;
    place-items: center;
    width: 56px;
    height: 56px;
    border-radius: 50%;
    color: var(--strong);
    background: var(--sidebar);
    transition: background 0.15s;
  }

  .round:hover {
    background: #3a3c42;
  }

  .round.off {
    color: var(--red);
    background: #fff;
  }

  .round.on {
    color: #fff;
    background: var(--green);
  }

  .round.leave {
    background: var(--red);
  }

  .round.leave:hover {
    background: #c03537;
  }

  .join {
    padding: 12px 24px;
    font-size: 15px;
  }
</style>
