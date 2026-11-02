<script lang="ts">
  import { ICONS } from '../lib/icons';
  let { name, size = 16, title = '' }: { name: string; size?: number; title?: string } = $props();
</script>

<span class="ico" style="width: {size}px; height: {size}px" title={title || undefined}>{@html ICONS[name] || ''}</span>
