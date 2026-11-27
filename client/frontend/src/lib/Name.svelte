<script lang="ts">
  import { people, type Profile } from './api.svelte'

  // profile substitui o salvo, para a prévia do editor de perfil.
  let { name, profile }: { name: string; profile?: Profile } = $props()

  const p = $derived(profile ?? people[name])
</script>

<span class="name {p?.effect ?? ''}" style:--c1={p?.color || null} style:--c2={p?.color2 || p?.color || null}>{name}</span>

<style>
  .name {
    color: var(--c1, inherit);
  }

  .gradient,
  .rainbow {
    background: linear-gradient(90deg, var(--c1, var(--accent)), var(--c2, #eb459e), var(--c1, var(--accent)));
    background-size: 200% 100%;
    background-clip: text;
    color: transparent;
    animation: slide 4s linear infinite;
  }

  .rainbow {
    background-image: linear-gradient(90deg, #ff5f6d, #ffc371, #47e891, #4fc3f7, #b388ff, #ff5f6d);
  }

  .glow {
    text-shadow:
      0 0 6px var(--c1, var(--accent)),
      0 0 14px var(--c2, var(--accent));
    animation: pulse 2.4s ease-in-out infinite;
  }

  @keyframes slide {
    to {
      background-position: 200% 0;
    }
  }

  @keyframes pulse {
    50% {
      text-shadow:
        0 0 2px var(--c1, var(--accent)),
        0 0 6px var(--c2, var(--accent));
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .name {
      animation: none;
    }
  }
</style>
