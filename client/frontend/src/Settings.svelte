<script lang="ts">
  import { auth, logout } from './lib/api.svelte'
  import { askPermission } from './lib/notify'
  import { settings } from './lib/settings.svelte'
  import { voice } from './lib/voice.svelte'
  import Icon from './lib/Icon.svelte'

  let { onclose, oninvite, onerror }: { onclose: () => void; oninvite: () => void; onerror: (err: unknown) => void } = $props()

  const denied = 'Notification' in window && Notification.permission === 'denied'
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="backdrop" role="presentation" onclick={(e) => e.target === e.currentTarget && onclose()}>
  <div class="modal" role="dialog" aria-modal="true" aria-labelledby="settings-title">
    <header>
      <h2 id="settings-title">Configurações</h2>
      <button class="icon-btn" title="Fechar" onclick={onclose}><Icon name="close" /></button>
    </header>

    <section>
      <h3>Voz</h3>
      <label class="row">
        <span>
          <strong>Supressão de ruído</strong>
          <small>Corta teclado, ventilador e barulho de fundo do seu microfone.</small>
          {#if voice.noiseError}<small class="fail">Não foi possível ativar: {voice.noiseError}</small>{/if}
        </span>
        <input type="checkbox" class="switch" checked={settings.noise} onchange={(e) => voice.setNoise(e.currentTarget.checked).catch(onerror)} />
      </label>
    </section>

    <section>
      <h3>Notificações</h3>
      <label class="row">
        <span>
          <strong>Notificações na área de trabalho</strong>
          <small>{denied ? 'Bloqueadas no Windows. Libere nas configurações de notificação do sistema.' : 'Avisa quando chega mensagem com o Concord em segundo plano.'}</small>
        </span>
        <input type="checkbox" class="switch" bind:checked={settings.notify} onchange={() => settings.notify && askPermission()} />
      </label>
      <label class="row">
        <span>
          <strong>Som de mensagem</strong>
          <small>Toca um aviso curto quando chega mensagem nova.</small>
        </span>
        <input type="checkbox" class="switch" bind:checked={settings.sound} />
      </label>
    </section>

    <section>
      <h3>Conta</h3>
      <div class="row">
        <span>
          <strong>{auth.session?.user.name}</strong>
          <small>{auth.session?.server}</small>
        </span>
        <div class="actions">
          <button class="btn" onclick={oninvite}>Gerar convite</button>
          <button class="btn danger" onclick={logout}>Sair</button>
        </div>
      </div>
    </section>
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
    animation: fade 0.15s ease-out;
  }

  .modal {
    width: min(520px, calc(100% - 32px));
    max-height: calc(100% - 64px);
    overflow-y: auto;
    padding: 20px 24px 8px;
    border-radius: var(--radius);
    background: var(--chat);
    box-shadow: 0 8px 40px #0008;
    animation: pop 0.18s ease-out;
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 8px;
  }

  h2 {
    margin: 0;
    font-size: 20px;
    color: var(--strong);
  }

  h3 {
    margin: 16px 0 4px;
    font-size: 12px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.02em;
    color: var(--muted);
  }

  section + section {
    border-top: 1px solid var(--line);
  }

  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 12px 0;
    cursor: pointer;
  }

  .row span {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .row strong {
    font-weight: 500;
    color: var(--strong);
  }

  .row small {
    color: var(--muted);
    font-size: 13px;
  }

  .row small.fail {
    color: var(--red);
  }

  .actions {
    display: flex;
    gap: 8px;
    flex: none;
  }

  .danger {
    background: var(--red);
  }

  .danger:hover {
    background: #c03537;
  }

  .switch {
    appearance: none;
    flex: none;
    position: relative;
    width: 40px;
    height: 24px;
    padding: 0;
    border-radius: 12px;
    background: var(--faint);
    cursor: pointer;
    transition: background 0.15s;
  }

  .switch::after {
    content: '';
    position: absolute;
    top: 3px;
    left: 3px;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: #fff;
    transition: transform 0.15s;
  }

  .switch:checked {
    background: var(--green);
  }

  .switch:checked::after {
    transform: translateX(16px);
  }

  @keyframes fade {
    from {
      opacity: 0;
    }
  }

  @keyframes pop {
    from {
      transform: scale(0.96);
      opacity: 0;
    }
  }
</style>
