<script lang="ts">
  import { onMount } from 'svelte'
  import { desktop } from './lib/desktop'

  let update = $state<{ version: string } | null>(null)
  let dismissed = $state(false)
  let installing = $state(false)
  let error = $state('')

  onMount(() => {
    if (!desktop) return
    const app = desktop
    // Sem internet ou com a API do GitHub limitada, tenta de novo na próxima rodada.
    const check = () => app.checkUpdate().then((u) => (update = u), () => {})
    // O servidor avisa na hora que sai release; checa de novo um minuto depois
    // porque o GitHub às vezes demora para apontar "latest" para a versão nova.
    const onRelease = () => {
      dismissed = false
      check()
      setTimeout(check, 60 * 1000)
    }
    check()
    const timer = setInterval(check, 30 * 60 * 1000)
    window.addEventListener('concord:release', onRelease)
    return () => {
      clearInterval(timer)
      window.removeEventListener('concord:release', onRelease)
    }
  })

  async function install() {
    installing = true
    error = ''
    try {
      await desktop!.installUpdate()
    } catch (err) {
      error = String(err)
      installing = false
    }
  }
</script>

{#if update && !dismissed}
  <aside class="update" role="status">
    <strong>Atualização disponível</strong>
    <p>A versão {update.version} do Concord está pronta. O app fecha, atualiza e abre de novo sozinho.</p>
    {#if error}<p class="fail">Não deu para atualizar: {error}</p>{/if}
    <div class="actions">
      <button class="later" disabled={installing} onclick={() => (dismissed = true)}>Depois</button>
      <button class="btn" disabled={installing} onclick={install}>{installing ? 'Baixando…' : 'Atualizar e reiniciar'}</button>
    </div>
  </aside>
{/if}

<style>
  .update {
    position: fixed;
    right: 16px;
    bottom: 16px;
    z-index: 30;
    width: 340px;
    padding: 16px;
    border-radius: var(--radius);
    background: var(--panel);
    box-shadow: 0 8px 32px #0009;
    border-left: 4px solid var(--green);
    animation: slide 0.25s ease-out;
  }

  strong {
    color: var(--strong);
  }

  p {
    margin: 6px 0 0;
    font-size: 14px;
    color: var(--muted);
  }

  .fail {
    color: var(--red);
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 14px;
  }

  .later {
    padding: 0 12px;
    color: var(--text);
    font-size: 14px;
  }

  .later:hover {
    text-decoration: underline;
  }

  .actions .btn {
    background: var(--green);
  }

  .actions .btn:hover {
    background: #1a8a4a;
  }

  @keyframes slide {
    from {
      transform: translateY(16px);
      opacity: 0;
    }
  }
</style>
