import { createResource, createSignal, Show, For } from 'solid-js';
import { apiGet, apiPost } from '../api.js';

export default function Sites(props) {
  const [sites, { refetch }] = createResource(() => apiGet('/api/sites'));
  const [creating, setCreating] = createSignal(false);
  const [error, setError] = createSignal('');
  const [form, setForm] = createSignal({
    niche: '',
    slug: '',
    tagline: '',
    description: '',
    domain: '',
    seed_keywords: '',
    affiliate_programs: 'amazon',
    ad_network: 'ezoic',
  });

  const submit = async (e) => {
    e.preventDefault();
    setError('');
    try {
      const f = form();
      await apiPost('/api/site/create', {
        niche: f.niche,
        slug: f.slug,
        tagline: f.tagline,
        description: f.description,
        domain: f.domain,
        seed_keywords: f.seed_keywords.split(',').map(s => s.trim()).filter(Boolean),
        affiliate_programs: f.affiliate_programs.split(',').map(s => s.trim()).filter(Boolean),
        ad_network: f.ad_network,
      });
      setCreating(false);
      refetch();
    } catch (e) { setError(e.message); }
  };

  const publish = async (id) => {
    try {
      const res = await apiPost('/api/site/publish', { site_id: id });
      alert(`Published to ${res.output_dir} (${res.article_count} articles)`);
    } catch (e) { alert(e.message); }
  };

  return (
    <div>
      <h2>Sites <button onClick={() => setCreating(!creating())}>{creating() ? 'Cancel' : '+ New site'}</button></h2>

      <Show when={creating()}>
        <div class="card">
          <h3>Create niche site</h3>
          <Show when={error()}><div class="error">{error()}</div></Show>
          <form onSubmit={submit}>
            <div class="form-row"><label>Niche</label><input value={form().niche} onInput={e => setForm({...form(), niche: e.target.value})} required placeholder="e.g. personal-finance-tools"/></div>
            <div class="form-row"><label>Slug (optional)</label><input value={form().slug} onInput={e => setForm({...form(), slug: e.target.value})} placeholder="auto-generated from niche"/></div>
            <div class="form-row"><label>Tagline</label><input value={form().tagline} onInput={e => setForm({...form(), tagline: e.target.value})} placeholder="Independent reviews of ..."/></div>
            <div class="form-row"><label>Description</label><input value={form().description} onInput={e => setForm({...form(), description: e.target.value})}/></div>
            <div class="form-row"><label>Domain (leave blank until registered)</label><input value={form().domain} onInput={e => setForm({...form(), domain: e.target.value})} placeholder="example.com"/></div>
            <div class="form-row"><label>Seed keywords (comma-separated)</label><input value={form().seed_keywords} onInput={e => setForm({...form(), seed_keywords: e.target.value})} placeholder="best budgeting app, free budget tracker, ..."/></div>
            <div class="form-row"><label>Affiliate programs</label><input value={form().affiliate_programs} onInput={e => setForm({...form(), affiliate_programs: e.target.value})}/></div>
            <div class="form-row"><label>Ad network</label><input value={form().ad_network} onInput={e => setForm({...form(), ad_network: e.target.value})}/></div>
            <button type="submit">Create site</button>
          </form>
        </div>
      </Show>

      <div class="card">
        <Show when={sites()?.length > 0} fallback={<div class="empty">No sites yet. Create one to start the engine.</div>}>
          <table>
            <thead><tr><th>Slug</th><th>Niche</th><th>Domain</th><th>Status</th><th>Health</th><th>Actions</th></tr></thead>
            <tbody>
              <For each={sites()}>{s => (
                <tr>
                  <td><a onClick={() => props.goTo('articles', s.id)} style="cursor:pointer;color:var(--accent)">{s.slug}</a></td>
                  <td class="muted">{s.niche}</td>
                  <td class="muted">{s.domain || <em>—</em>}</td>
                  <td><span class={`pill pill-${s.status}`}>{s.status}</span></td>
                  <td>{(s.health_score * 100).toFixed(0)}%</td>
                  <td>
                    <button class="btn-secondary" onClick={() => publish(s.id)}>Publish</button>
                  </td>
                </tr>
              )}</For>
            </tbody>
          </table>
        </Show>
      </div>
    </div>
  );
}
