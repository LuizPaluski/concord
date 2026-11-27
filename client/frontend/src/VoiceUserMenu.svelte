<script lang="ts">
  import { DEFAULT_LISTEN, settings } from './lib/settings.svelte'
  import { voice } from './lib/voice.svelte'
  import Name from './lib/Name.svelte'

  let { name, x, y }: { name: string; x: number; y: number } = $props()

  const l = $derived(settings.listen[name] ?? DEFAULT_LISTEN)

  function set(change: Partial<typeof DEFAULT_LISTEN>) {
    settings.listen[name] = { ...l, ...change }
    voice.applyListen(name)
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="menu" role="menu" tabindex="-1" style:left="{Math.min(x, innerWidth - 236)}px" style:top="{Math.min(y, innerHeight - 220)}px" onclick={(e) => e.stopPropagation()}>
  <strong><Name {name} /></strong>
  <label class="volume">
    <span>Volume do usuário <em>{l.volume}%</em></span>
    <input type="range" min="0" max="100" step="5" value={l.volume} oninput={(e) => set({ volume: Number(e.currentTarget.value) })} />
  </label>
  <label class="check">
    Silenciar
    <input type="checkbox" checked={l.muted} onchange={(e) => set({ muted: e.currentTarget.checked })} />
  </label>
  <label class="check">
    Silenciar som da transmissão
    <input type="checkbox" checked={l.screenMuted} onchange={(e) => set({ screenMuted: e.currentTarget.checked })} />
  </label>
  <small>Só você ouve diferente.</small>
</div>

<style>
  .menu {
    position: fixed;
    z-index: 15;
    display: flex;
    flex-direction: column;
    gap: 4px;
    width: 220px;
    padding: 10px;
    border-radius: var(--radius);
    background: var(--rail);
    box-shadow: 0 8px 24px #0007;
    animation: pop 0.12s ease-out;
  }

  strong {
    padding: 0 6px 6px;
    font-size: 14px;
    color: var(--strong);
    border-bottom: 1px solid var(--line);
  }

  .volume {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 6px;
    font-size: 13px;
    color: var(--muted);
  }

  .volume span {
    display: flex;
    justify-content: space-between;
  }

  .volume em {
    font-style: normal;
    color: var(--text);
  }

  input[type='range'] {
    padding: 0;
    accent-color: var(--accent);
    background: none;
  }

  .check {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px;
    border-radius: 4px;
    font-size: 14px;
    color: var(--text);
    cursor: pointer;
  }

  .check:hover {
    background: var(--hover);
  }

  .check input {
    width: 16px;
    height: 16px;
    accent-color: var(--accent);
  }

  small {
    padding: 4px 6px 0;
    font-size: 12px;
    color: var(--faint);
  }
</style>
