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

  function switchMode() {
    mode = mode === 'login' ? 'register' : 'login'
    error = ''
  }
</script>

<main>
  <form onsubmit={submit}>
    <h1>{mode === 'login' ? 'Que bom te ver de novo!' : 'Criar uma conta'}</h1>
    <p class="sub">{mode === 'login' ? 'Entre para continuar a conversa.' : 'Você precisa do convite que alguém do grupo te mandou.'}</p>

    <label>Servidor <input bind:value={server} placeholder="chat.exemplo.com" required /></label>
    {#if mode === 'register'}
      <label>Convite <input bind:value={invite} required autocomplete="off" /></label>
    {/if}
    <label>Nome de usuário <input bind:value={name} required minlength="2" maxlength="32" autocomplete="username" /></label>
    <label>
      Senha
      <input type="password" bind:value={password} required minlength="8" maxlength="72" autocomplete={mode === 'login' ? 'current-password' : 'new-password'} />
    </label>

    {#if error}<p class="error" role="alert">{error}</p>{/if}

    <button class="btn" disabled={busy}>{mode === 'login' ? 'Entrar' : 'Continuar'}</button>
    <p class="swap">
      {mode === 'login' ? 'Recebeu um convite?' : 'Já tem uma conta?'}
      <button type="button" onclick={switchMode}>{mode === 'login' ? 'Criar conta' : 'Entrar'}</button>
    </p>
  </form>
</main>

<style>
  main {
    height: 100%;
    display: grid;
    place-items: center;
    background:
      radial-gradient(70rem 50rem at 0% 100%, #5865f2cc, transparent 60%),
      radial-gradient(60rem 40rem at 100% 0%, #eb459e66, transparent 60%),
      #404eed;
  }

  form {
    width: min(480px, calc(100% - 32px));
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 32px;
    border-radius: var(--radius);
    background: var(--chat);
    box-shadow: 0 2px 10px #0003;
  }

  h1 {
    margin: 0;
    text-align: center;
    font-size: 24px;
    font-weight: 600;
    color: var(--strong);
  }

  .sub {
    margin: -8px 0 4px;
    text-align: center;
    color: var(--muted);
  }

  label {
    display: flex;
    flex-direction: column;
    gap: 8px;
    font-size: 12px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.02em;
    color: var(--muted);
  }

  label input {
    font-size: 16px;
    font-weight: 400;
    text-transform: none;
    letter-spacing: 0;
    color: var(--text);
  }

  .error {
    margin: 0;
    color: var(--red);
    font-size: 14px;
  }

  .btn {
    margin-top: 4px;
    padding: 12px;
    font-size: 16px;
  }

  .swap {
    margin: -4px 0 0;
    font-size: 14px;
    color: var(--faint);
  }

  .swap button {
    color: #00a8fc;
  }

  .swap button:hover {
    text-decoration: underline;
  }
</style>
