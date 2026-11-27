<script lang="ts">
  import { hue, mediaUrl, type Profile } from './lib/api.svelte'
  import Avatar from './lib/Avatar.svelte'
  import Name from './lib/Name.svelte'

  let { profile, online = false }: { profile: Profile; online?: boolean } = $props()

  const bannerColor = $derived(profile.color || `hsl(${hue(profile.name)} 45% 45%)`)
</script>

<article class="card" style:--banner={bannerColor} style:--banner2={profile.color2 || bannerColor}>
  <div class="banner">
    {#if profile.banner}<img src={mediaUrl(profile.banner)} alt="" />{/if}
  </div>
  <div class="pic"><Avatar name={profile.name} {profile} size={80} {online} ring="var(--panel)" /></div>
  <div class="body">
    <h3><Name name={profile.name} {profile} /></h3>
    {#if profile.bio}
      <div class="about">
        <h4>Sobre mim</h4>
        <p>{profile.bio}</p>
      </div>
    {/if}
  </div>
</article>

<style>
  .card {
    width: 300px;
    overflow: hidden;
    border-radius: var(--radius);
    background: var(--panel);
    box-shadow: 0 8px 24px #0006;
  }

  .banner {
    height: 106px;
    background: linear-gradient(135deg, var(--banner), var(--banner2));
  }

  .banner img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .pic {
    margin: -44px 0 0 16px;
    width: max-content;
    padding: 6px;
    border-radius: 50%;
    background: var(--panel);
  }

  .body {
    margin: 8px 12px 12px;
    padding: 12px;
    border-radius: var(--radius);
    background: var(--rail);
  }

  h3 {
    margin: 0;
    font-size: 20px;
    font-weight: 700;
    color: var(--strong);
  }

  .about {
    margin-top: 12px;
    padding-top: 12px;
    border-top: 1px solid var(--line);
  }

  h4 {
    margin: 0 0 6px;
    font-size: 12px;
    font-weight: 700;
    text-transform: uppercase;
    color: var(--strong);
  }

  p {
    margin: 0;
    font-size: 14px;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
</style>
