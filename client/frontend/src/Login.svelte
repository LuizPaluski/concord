<script lang="ts">
  import { enter } from './lib/api.svelte'

  let mode = $state<'login' | 'register'>('login')
  let server = $state(localStorage.getItem('concord.server') ?? '')
  let name = $state('')
  let password = $state('')
  let invite = $state('')
  let error = $state('')
  let busy = $state(false)

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    error = ''
    try {
      await enter(server, mode, { name, password, invite })
    } catch (err) {
      error = (err as Error).message || 'não foi possível conectar ao servidor'
    } finally {
      busy = false
    }
  }
</script>

<main>
  <form onsubmit={submit}>
    <h1>Concord</h1>
    <p class="sub">{mode === 'login' ? 'Bem-vindo de volta.' : 'Entre com o convite que te mandaram.'}</p>

    <label>Servidor <input bind:value={server} placeholder="chat.seudominio.com" required /></label>
    {#if mode === 'register'}
      <label>Convite <input bind:value={invite} required autocomplete="off" /></label>
    {/if}
    <label>Nome <input bind:value={name} required minlength="2" maxlength="32" autocomplete="username" /></label>
    <label>
      Senha
      <input type="password" bind:value={password} required minlength="8" maxlength="72" autocomplete={mode === 'login' ? 'current-password' : 'new-password'} />
    </label>

    {#if error}<p class="error" role="alert">{error}</p>{/if}

    <button class="primary" disabled={busy}>{mode === 'login' ? 'Entrar' : 'Criar conta'}</button>
    <button type="button" class="link" onclick={() => ((mode = mode === 'login' ? 'register' : 'login'), (error = ''))}>
      {mode === 'login' ? 'Tenho um convite' : 'Já tenho conta'}
    </button>
  </form>
</main>

<style>
  main {
    height: 100%;
    display: grid;
    place-items: center;
    background:
      radial-gradient(60rem 40rem at 10% -10%, #5eead41a, transparent 60%),
      radial-gradient(50rem 40rem at 110% 110%, #818cf81a, transparent 60%),
      var(--bg);
  }

  form {
    width: min(380px, calc(100% - 32px));
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 32px;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 16px;
    box-shadow: 0 30px 80px #0008;
  }

  h1 {
    margin: 0;
    font-size: 28px;
    letter-spacing: -0.02em;
  }

  .sub {
    margin: -8px 0 6px;
    color: var(--muted);
  }

  label {
    display: flex;
    flex-direction: column;
    gap: 6px;
    font-size: 13px;
    color: var(--muted);
  }

  .error {
    margin: 0;
    color: var(--danger);
    font-size: 14px;
  }

  .primary {
    margin-top: 6px;
    padding: 11px;
    border-radius: 8px;
    font-weight: 600;
    background: var(--accent);
    color: var(--accent-ink);
  }

  .primary:disabled {
    opacity: 0.6;
  }

  .link {
    color: var(--muted);
    font-size: 14px;
  }

  .link:hover {
    color: var(--text);
  }
</style>
