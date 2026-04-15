import { createResource, Show, For } from 'solid-js';
import { apiGet } from '../api.js';

export default function Revenue() {
  const [summary] = createResource(() => apiGet('/api/revenue/summary?days=30'));
  const [cost] = createResource(() => apiGet('/api/revenue/cost-report?days=30'));

  return (
    <div>
      <h2>Revenue (last 30 days)</h2>
      <div class="card">
        <Show when={(summary()?.sites || []).length > 0} fallback={<div class="empty">No revenue data yet. Cowork ingests via POST /api/revenue/ingest.</div>}>
          <table>
            <thead><tr><th>Site</th><th>Revenue</th><th>Sessions</th><th>RPM</th></tr></thead>
            <tbody>
              <For each={summary().sites}>{s => (
                <tr>
                  <td>{s.site_slug}</td>
                  <td>${s.revenue_usd.toFixed(2)}</td>
                  <td>{s.sessions.toLocaleString()}</td>
                  <td>${s.sessions > 0 ? (s.revenue_usd / s.sessions * 1000).toFixed(2) : '—'}</td>
                </tr>
              )}</For>
            </tbody>
          </table>
        </Show>
      </div>

      <div class="card">
        <h3>ROI (30d)</h3>
        <div class="kpi-grid">
          <div class="kpi"><div class="kpi-label">Revenue</div><div class="kpi-value">${(cost()?.revenue_usd || 0).toFixed(2)}</div></div>
          <div class="kpi"><div class="kpi-label">Claude cost</div><div class="kpi-value">${(cost()?.cost_usd || 0).toFixed(2)}</div></div>
          <div class="kpi"><div class="kpi-label">ROI</div><div class="kpi-value">{(cost()?.roi || 0).toFixed(1)}×</div></div>
          <div class="kpi"><div class="kpi-label">Budget cap</div><div class="kpi-value">${cost()?.budget_cap_usd?.toFixed(2) || '—'}</div><div class="kpi-sub">MTD ${cost()?.month_to_date_usd?.toFixed(2) || '0.00'}</div></div>
        </div>
      </div>
    </div>
  );
}
