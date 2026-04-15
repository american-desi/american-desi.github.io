import { createResource, createSignal, Show, For } from 'solid-js';
import { apiGet, apiPost } from '../api.js';

export default function Articles(props) {
  const [sites] = createResource(() => apiGet('/api/sites'));
  const [siteId, setSiteId] = createSignal(props.siteId || '');
  const [articles, { refetch }] = createResource(siteId, id => id ? apiGet('/api/articles/' + id) : Promise.resolve([]));
  const [generating, setGenerating] = createSignal(false);
  const [form, setForm] = createSignal({ focus_keyword: '', article_type: 'article', cluster: '' });
  const [error, setError] = createSignal('');

  const generate = async (e) => {
    e.preventDefault();
    setError('');
    try {
      const f = form();
      await apiPost('/api/generate', {
        site_id: siteId(),
        focus_keyword: f.focus_keyword,
        article_type: f.article_type,
        cluster: f.cluster,
      });
      setGenerating(false);
      refetch();
    } catch (e) { setError(e.message); }
  };

  return (
    <div>
      <h2>Articles</h2>

      <div class="form-row" style="max-width:440px">
        <label>Site</label>
        <select onChange={e => setSiteId(e.target.value)} value={siteId()}>
          <option value="">— choose a site —</option>
          <For each={sites()}>{s => <option value={s.id}>{s.slug}</option>}</For>
        </select>
      </div>

      <Show when={siteId()}>
        <button onClick={() => setGenerating(!generating())}>{generating() ? 'Cancel' : '+ Generate article'}</button>

        <Show when={generating()}>
          <div class="card" style="margin-top:1rem">
            <h3>Generate article</h3>
            <Show when={error()}><div class="error">{error()}</div></Show>
            <form onSubmit={generate}>
              <div class="form-row"><label>Focus keyword</label><input required value={form().focus_keyword} onInput={e => setForm({...form(), focus_keyword: e.target.value})}/></div>
              <div class="form-row"><label>Article type</label>
                <select value={form().article_type} onChange={e => setForm({...form(), article_type: e.target.value})}>
                  <option value="article">Article</option>
                  <option value="review">Review</option>
                  <option value="comparison">Comparison</option>
                  <option value="pillar">Pillar</option>
                </select>
              </div>
              <div class="form-row"><label>Cluster (optional)</label><input value={form().cluster} onInput={e => setForm({...form(), cluster: e.target.value})}/></div>
              <button type="submit">Generate (blocks ~30s)</button>
            </form>
          </div>
        </Show>

        <div class="card" style="margin-top:1rem">
          <Show when={articles()?.length > 0} fallback={<div class="empty">No articles yet.</div>}>
            <table>
              <thead><tr><th>Title</th><th>Focus</th><th>Words</th><th>Status</th><th>Published</th></tr></thead>
              <tbody>
                <For each={articles()}>{a => (
                  <tr>
                    <td>{a.title}</td>
                    <td class="muted">{a.focus_keyword}</td>
                    <td>{a.word_count}</td>
                    <td><span class={`pill pill-${a.status}`}>{a.status}</span></td>
                    <td class="muted">{a.published_at ? new Date(a.published_at).toLocaleDateString() : '—'}</td>
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
