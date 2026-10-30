<script lang="ts">
  import { get } from '../lib/api';
  import { dateTime } from '../lib/format';
  let { id }: { id: string } = $props();
  let rows: any[] = $state([]);
  get(`/api/change-orders/${id}/history`).then((r) => (rows = r)).catch(() => {});
  const labels: Record<string, string> = {
    'co.create': 'Created', 'co.update': 'Edited', 'co.item.add': 'Item added', 'co.item.remove': 'Item removed', 'co.approvers': 'Approvers changed',
    'co.submit': 'Submitted for review', 'co.withdraw': 'Returned to Draft', 'co.decision': 'Decision', 'co.state': 'State changed',
    'co.cancel': 'Cancelled', 'co.release': 'Released', 'signature.failed': 'Signature failed', 'record.lock': 'Access list changed',
  };
  function detail(r: any) {
    const d = r.detail || {};
    if (d.item) return `${d.item}${d.action ? ' (' + d.action + ')' : ''}`;
    if (d.decision) return `${d.decision}${d.comment ? ': ' + d.comment : ''}`;
    if (d.from && d.to) return `${d.from} to ${d.to}`;
    if (d.baseline) return d.baseline;
    return '';
  }
</script>

<div class="card table-wrap">
  <table class="data"><thead><tr><th>When</th><th>Who</th><th>Event</th><th>Detail</th></tr></thead><tbody>
    {#each rows as r}<tr><td style="white-space: nowrap">{dateTime(r.at)}</td><td>{r.username}</td><td>{labels[r.action] || r.action}</td><td class="small">{detail(r)}</td></tr>
    {:else}<tr><td colspan="4" class="empty">No history.</td></tr>{/each}
  </tbody></table>
</div>
