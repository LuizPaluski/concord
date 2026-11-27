<script lang="ts">
  import { hue, mediaUrl, people, type Profile } from './api.svelte'

  let { name, size = 40, online = false, ring, profile }: { name: string; size?: number; online?: boolean; ring?: string; profile?: Profile } = $props()

  const avatar = $derived((profile ?? people[name])?.avatar)
</script>

<span class="avatar" class:online style:--h={hue(name)} style:--size="{size}px" style:--ring={ring}>
  {#if avatar}<img src={mediaUrl(avatar)} alt="" loading="lazy" />{:else}{name[0]}{/if}
</span>

<style>
  img {
    width: 100%;
    height: 100%;
    border-radius: 50%;
    object-fit: cover;
  }
</style>
