import { createResource, Show, For } from 'solid-js';
import { apiGet, apiPost } from '../api.js';

export default function Analytics() {
  const [summary, { refetch }] = createResource(() => apiGet('/api/analytics/summary'));

  const pull = async () => {
    try {
      const res = await apiPost('/api/analytics/pull?days=28', {});
      alert(`Ingested ${res.rows_ingested} rows`);
      refetch();
    } catch (e) { alert(e.message); }
  };

  return (
    <div>
      <h2>SEO Analytics <button class="btn-secondary" onClick={pull}>Pull from GSC</button></h2>
      <Show when={summary()} fallback={<div class="loader">Loading…</div>}>
        <div class="card">
          <h3>Striking distance (position 5-20)</h3>
          <p class="muted" style="margin-bottom:0.5rem">High-ROI optimization targets.</p>
          <Section rows={summary().striking_distance}/>
        </div>
        <div class="card">
          <h3>Low CTR (&lt; 2% with &ge; 500 impressions)</h3>
          <p class="muted" style="margin-bottom:0.5rem">Meta rewrite candidates.</p>
          <Section rows={summary().low_ctr}/>
        </div>
        <div class="card">
          <h3>Declining (last 14d &lt; 60% of prior 14d)</h3>
          <p class="muted" style="margin-bottom:0.5rem">Content refresh candidates.</p>
          <Section rows={summary().declining}/>
        </div>
        <div class="card">
          <h3>Cannibalization</h3>
          <p class="muted" style="margin-bottom:0.5rem">Multiple articles competing for the same query.</p>
          <Show when={(summary().cannibalization || []).length > 0} fallback={<div class="empty">None detected.</div>}>
            <table>
              <thead><tr><th>Site</th><th>Query</th><th>Article IDs</th></tr></thead>
              <tbody>
                <For each={summary().cannibalization}>{c => (
                  <tr>
                    <td class="muted">{c.site_id.slice(0,8)}…</td>
                    <td>{c.query}</td>
                    <td class="muted">{c.article_ids.join(', ')}</td>
                  </tr>
                )}</For>
              </tbody>
            </table>
          </Show>
        </div>
      </Show>
    </div>
  );
}

function Section(props) {
  return (
    <Show when={(props.rows || []).length > 0} fallback={<div class="empty">None.</div>}>
      <table>
        <thead><tr><th>Article</th><th>Impressions</th><th>Clicks</th><th>CTR</th><th>Position</th><th>Top query</th></tr></thead>
        <tbody>
          <For each={props.rows}>{m => (
            <tr>
              <td class="muted"><code>{m.ArticleID.slice(0, 8)}</code></td>
              <td>{m.Impressions.toLocaleString()}</td>
              <td>{m.Clicks.toLocaleString()}</td>
              <td>{(m.CTR * 100).toFixed(2)}%</td>
              <td>{m.AvgPosition.toFixed(1)}</td>
              <td>{m.TopQuery}</td>
            </tr>
          )}</For>
        </tbody>
      </table>
    </Show>
  );
}
