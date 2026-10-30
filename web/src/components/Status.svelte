<script lang="ts">
  let { value }: { value: string } = $props();
</script>

{#if value}<span class="status {value.replace(/ /g, '-')}">{value}</span>{/if}
