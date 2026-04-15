import { createResource, createSignal, Show, For } from 'solid-js';
import { apiGet, apiPost } from '../api.js';

export default function Niches() {
  const [niches, { refetch }] = createResource(() => apiGet('/api/niche/list'));
  const [analyzing, setAnalyzing] = createSignal(false);
  const [form, setForm] = createSignal({ niche: '', seed_keywords: '' });
  const [error, setError] = createSignal('');

  const submit = async (e) => {
    e.preventDefault();
    setError('');
    setAnalyzing(true);
    try {
      const f = form();
      await apiPost('/api/niche/analyze', {
        niche: f.niche,
        seed_keywords: f.seed_keywords.split(',').map(s => s.trim()).filter(Boolean),
      });
      setForm({ niche: '', seed_keywords: '' });
      refetch();
    } catch (e) { setError(e.message); }
    finally { setAnalyzing(false); }
  };

  return (
    <div>
      <h2>Niche analyzer</h2>
      <div class="card">
        <h3>Analyze new niche</h3>
        <Show when={error()}><div class="error">{error()}</div></Show>
        <form onSubmit={submit}>
          <div class="form-row"><label>Niche</label><input required value={form().niche} onInput={e => setForm({...form(), niche: e.target.value})} placeholder="e.g. outdoor-kitchen-equipment"/></div>
          <div class="form-row"><label>Seed keywords (comma-separated, optional)</label><input value={form().seed_keywords} onInput={e => setForm({...form(), seed_keywords: e.target.value})}/></div>
          <button type="submit" disabled={analyzing()}>{analyzing() ? 'Analyzing…' : 'Analyze'}</button>
        </form>
      </div>

      <div class="card">
        <h3>Scored niches</h3>
        <Show when={niches()?.length > 0} fallback={<div class="empty">No niches analyzed yet.</div>}>
          <table>
            <thead><tr><th>Niche</th><th>Score</th><th>Volume</th><th>Competition</th><th>Affiliate %</th><th>RPM</th><th>Velocity</th><th>Time to rev</th></tr></thead>
            <tbody>
              <For each={niches()}>{n => (
                <tr title={n.rationale}>
                  <td>{n.niche}</td>
                  <td style={{color: scoreColor(n.score), fontWeight: 600}}>{n.score.toFixed(1)}</td>
                  <td>{n.monthly_search_vol.toLocaleString()}</td>
                  <td><span class={`pill pill-${n.competition_level === 'low' ? 'live' : n.competition_level === 'high' ? 'failed' : 'queued'}`}>{n.competition_level}</span></td>
                  <td>{n.avg_affiliate_comm.toFixed(1)}</td>
                  <td>${n.est_rpm.toFixed(1)}</td>
                  <td>{n.content_velocity}</td>
                  <td class="muted">{n.time_to_revenue}</td>
                </tr>
              )}</For>
            </tbody>
          </table>
        </Show>
      </div>
    </div>
  );
}

function scoreColor(s) {
  if (s >= 8) return 'var(--success)';
  if (s >= 6) return 'var(--warn)';
  return 'var(--danger)';
}
