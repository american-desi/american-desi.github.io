import { createResource, createSignal, Show, For } from 'solid-js';
import { apiGet, apiPost } from '../api.js';

export default function Optimizer() {
  const [log, { refetch }] = createResource(() => apiGet('/api/optimize/log'));
  const [running, setRunning] = createSignal(false);
  const [force, setForce] = createSignal(false);

  const run = async () => {
    setRunning(true);
    try {
      const url = '/api/optimize/run' + (force() ? '?force=true' : '');
      const res = await apiPost(url, {});
      alert(`Cycle done: ${res.rewrites} rewrites, ${res.meta_refreshes} meta, ${res.content_refreshes} refresh, $${res.cost_usd.toFixed(4)}`);
      refetch();
    } catch (e) { alert(e.message); }
    finally { setRunning(false); }
  };

  return (
    <div>
      <h2>Self-healing optimizer</h2>
      <div class="card">
        <p>The optimizer runs automatically every {'\u007E'}7 days. Use "Run now" to trigger ad-hoc.</p>
        <p style="margin-top:0.5rem"><label><input type="checkbox" style="width:auto;margin-right:0.4rem" checked={force()} onChange={e => setForce(e.target.checked)}/> Force (bypass 60-day minimum age)</label></p>
        <button onClick={run} disabled={running()} style="margin-top:0.8rem">{running() ? 'Running…' : 'Run cycle now'}</button>
      </div>

      <div class="card">
        <h3>Recent activity</h3>
        <Show when={log()?.length > 0} fallback={<div class="empty">No optimizations logged yet.</div>}>
          <table>
            <thead><tr><th>When</th><th>Article</th><th>Kind</th><th>Reason</th><th>Cost</th></tr></thead>
            <tbody>
              <For each={log()}>{o => (
                <tr>
                  <td class="muted">{new Date(o.created_at).toLocaleString()}</td>
                  <td class="muted"><code>{o.article_id.slice(0, 8)}</code></td>
                  <td><span class="pill pill-queued">{o.kind}</span></td>
                  <td class="muted">{o.reason}</td>
                  <td>${o.cost_usd.toFixed(4)}</td>
                </tr>
              )}</For>
            </tbody>
          </table>
        </Show>
      </div>
    </div>
  );
}
