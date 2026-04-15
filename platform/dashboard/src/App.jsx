import { createSignal, Show, onMount } from 'solid-js';
import Overview from './views/Overview.jsx';
import Sites from './views/Sites.jsx';
import Articles from './views/Articles.jsx';
import Pipeline from './views/Pipeline.jsx';
import Analytics from './views/Analytics.jsx';
import Optimizer from './views/Optimizer.jsx';
import Revenue from './views/Revenue.jsx';
import Niches from './views/Niches.jsx';

export default function App() {
  const [view, setView] = createSignal('overview');
  const [selectedSite, setSelectedSite] = createSignal(null);

  const go = (v, ctx = null) => {
    setView(v);
    if (ctx) setSelectedSite(ctx);
  };

  return (
    <div class="app">
      <aside class="sidebar">
        <h1>AI Content Platform</h1>
        <a class={`nav-item ${view() === 'overview' ? 'active' : ''}`} onClick={() => go('overview')}>Overview</a>
        <a class={`nav-item ${view() === 'sites' ? 'active' : ''}`} onClick={() => go('sites')}>Sites</a>
        <a class={`nav-item ${view() === 'articles' ? 'active' : ''}`} onClick={() => go('articles')}>Articles</a>
        <a class={`nav-item ${view() === 'pipeline' ? 'active' : ''}`} onClick={() => go('pipeline')}>Pipeline</a>
        <a class={`nav-item ${view() === 'analytics' ? 'active' : ''}`} onClick={() => go('analytics')}>SEO Analytics</a>
        <a class={`nav-item ${view() === 'optimizer' ? 'active' : ''}`} onClick={() => go('optimizer')}>Self-Healing</a>
        <a class={`nav-item ${view() === 'revenue' ? 'active' : ''}`} onClick={() => go('revenue')}>Revenue</a>
        <a class={`nav-item ${view() === 'niches' ? 'active' : ''}`} onClick={() => go('niches')}>Niches</a>
      </aside>
      <main>
        <Show when={view() === 'overview'}><Overview goTo={go} /></Show>
        <Show when={view() === 'sites'}><Sites goTo={go} /></Show>
        <Show when={view() === 'articles'}><Articles siteId={selectedSite()} /></Show>
        <Show when={view() === 'pipeline'}><Pipeline /></Show>
        <Show when={view() === 'analytics'}><Analytics /></Show>
        <Show when={view() === 'optimizer'}><Optimizer /></Show>
        <Show when={view() === 'revenue'}><Revenue /></Show>
        <Show when={view() === 'niches'}><Niches /></Show>
      </main>
    </div>
  );
}
