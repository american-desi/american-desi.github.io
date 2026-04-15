import { createResource, Show, For } from 'solid-js';
import { apiGet, apiPost } from '../api.js';

export default function Pipeline() {
  const [items, { refetch }] = createResource(() => apiGet('/api/pipeline/status'));

  const tick = async () => {
    try {
      const res = await apiPost('/api/pipeline/tick', {});
      alert(`Processed ${res.processed} items`);
      refetch();
    } catch (e) { alert(e.message); }
  };

  return (
    <div>
      <h2>Content pipeline <button class="btn-secondary" onClick={tick}>Run now</button></h2>
      <div class="card">
        <Show when={items()?.length > 0} fallback={<div class="empty">Pipeline is empty. Sites with generated keyword maps will auto-populate.</div>}>
          <table>
            <thead><tr><th>Keyword</th><th>Type</th><th>Scheduled</th><th>Status</th><th>Attempts</th><th>Last error</th></tr></thead>
            <tbody>
              <For each={items()}>{p => (
                <tr>
                  <td>{p.focus_keyword}</td>
                  <td class="muted">{p.article_type}</td>
                  <td class="muted">{new Date(p.scheduled_for).toLocaleString()}</td>
                  <td><span class={`pill pill-${p.status}`}>{p.status}</span></td>
                  <td>{p.attempts}</td>
                  <td class="muted" style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{p.last_error}</td>
                </tr>
              )}</For>
            </tbody>
          </table>
        </Show>
      </div>
    </div>
  );
}
