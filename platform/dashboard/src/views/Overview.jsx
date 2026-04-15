import { createResource, Show, For } from 'solid-js';
import { apiGet } from '../api.js';

export default function Overview(props) {
  const [sites] = createResource(() => apiGet('/api/sites'));
  const [revenue] = createResource(() => apiGet('/api/revenue/summary?days=30'));
  const [cost] = createResource(() => apiGet('/api/revenue/cost-report?days=30'));
  const [pipeline] = createResource(() => apiGet('/api/pipeline/status'));

  const liveSites = () => (sites() || []).filter(s => s.status === 'live').length;
  const totalSites = () => (sites() || []).length;
  const totalRev = () => (revenue()?.sites || []).reduce((a, s) => a + s.revenue_usd, 0);
  const mtd = () => cost()?.month_to_date_usd || 0;
  const roi = () => cost()?.roi || 0;
  const queued = () => (pipeline() || []).filter(p => p.status === 'queued').length;

  return (
    <div>
      <h2>Portfolio overview</h2>
      <div class="kpi-grid">
        <div class="kpi">
          <div class="kpi-label">Sites</div>
          <div class="kpi-value">{totalSites()}</div>
          <div class="kpi-sub">{liveSites()} live</div>
        </div>
        <div class="kpi">
          <div class="kpi-label">Revenue (30d)</div>
          <div class="kpi-value">${totalRev().toFixed(2)}</div>
          <div class="kpi-sub">across all sites</div>
        </div>
        <div class="kpi">
          <div class="kpi-label">Claude spend (MTD)</div>
          <div class="kpi-value">${mtd().toFixed(2)}</div>
          <div class="kpi-sub">budget ${cost()?.budget_cap_usd?.toFixed(2) || '…'}</div>
        </div>
        <div class="kpi">
          <div class="kpi-label">ROI (30d)</div>
          <div class="kpi-value">{roi().toFixed(1)}×</div>
          <div class="kpi-sub">revenue ÷ cost</div>
        </div>
        <div class="kpi">
          <div class="kpi-label">Queued articles</div>
          <div class="kpi-value">{queued()}</div>
          <div class="kpi-sub">in pipeline</div>
        </div>
      </div>

      <div class="card">
        <h3>Revenue per site (30d)</h3>
        <Show when={(revenue()?.sites || []).length > 0} fallback={<div class="empty">No revenue data yet.</div>}>
          <table>
            <thead><tr><th>Site</th><th>Revenue</th><th>Sessions</th><th>Clicks</th><th>Conversions</th></tr></thead>
            <tbody>
              <For each={revenue().sites}>{s => (
                <tr>
                  <td>{s.site_slug}</td>
                  <td>${s.revenue_usd.toFixed(2)}</td>
                  <td>{s.sessions.toLocaleString()}</td>
                  <td>{s.clicks.toLocaleString()}</td>
                  <td>{s.conversions.toLocaleString()}</td>
                </tr>
              )}</For>
            </tbody>
          </table>
        </Show>
      </div>

      <div class="card">
        <h3>Claude spend by purpose (30d)</h3>
        <Show when={(cost()?.spend_by_purpose || []).length > 0} fallback={<div class="empty">No Claude API calls yet.</div>}>
          <table>
            <thead><tr><th>Purpose</th><th>Requests</th><th>Cost USD</th></tr></thead>
            <tbody>
              <For each={cost().spend_by_purpose}>{s => (
                <tr>
                  <td>{s.purpose}</td>
                  <td>{s.requests}</td>
                  <td>${s.cost_usd.toFixed(4)}</td>
                </tr>
              )}</For>
            </tbody>
          </table>
        </Show>
      </div>
    </div>
  );
}
