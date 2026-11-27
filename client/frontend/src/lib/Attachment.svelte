<script lang="ts">
  import { fileUrl, type Attachment } from './api.svelte'

  let { file, onview }: { file: Attachment; onview: (url: string) => void } = $props()

  const url = $derived(fileUrl(file.id))
</script>

{#if file.mime.startsWith('video/')}
  <!-- svelte-ignore a11y_media_has_caption -->
  <video class="media" src={url} controls preload="metadata"></video>
{:else}
  <button class="media" aria-label="Abrir {file.name || 'imagem'}" onclick={() => onview(url)}>
    <img src={url} alt={file.name} loading="lazy" />
  </button>
{/if}

<style>
  .media {
    display: block;
    margin-top: 4px;
    max-width: min(400px, 100%);
    max-height: 300px;
    border-radius: var(--radius);
    background: var(--rail);
  }

  button.media {
    overflow: hidden;
    cursor: zoom-in;
  }

  img {
    display: block;
    max-width: 100%;
    max-height: 300px;
    object-fit: contain;
  }
</style>
