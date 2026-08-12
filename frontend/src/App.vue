<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import {
  Activity,
  ArrowDownToLine,
  ArrowUpRight,
  Box,
  Check,
  ChevronDown,
  CircleAlert,
  CircleDot,
  CircleHelp,
  Clock3,
  Code2,
  Command,
  Database,
  GitBranch,
  Globe2,
  Hammer,
  LayoutDashboard,
  LifeBuoy,
  ListChecks,
  Moon,
  MoreHorizontal,
  PackageCheck,
  Play,
  Plus,
  RotateCw,
  Search,
  ServerCog,
  Settings2,
  ShieldCheck,
  Sparkles,
  Sun,
  TerminalSquare,
  UploadCloud,
  Webhook,
  X,
  Zap,
} from '@lucide/vue';

type HealthState = 'checking' | 'healthy' | 'offline';
type Site = {
  id: string;
  name: string;
  hostname: string;
  project_root: string;
  web_root: string;
  framework: string;
  strategy: string;
  status: string;
  runtime?: { permissions?: { status: string; deployable: boolean } };
};
type DiscoveredSite = Site & {
  detection: { confidence: string; evidence: string[]; document_root: string };
  permissions: { status: string; readable: boolean; writable: boolean; deployable: boolean; message: string };
  source: string;
  already_managed: boolean;
};

const healthState = ref<HealthState>('checking');
const isDark = ref(true);
const commandOpen = ref(false);
const commandQuery = ref('');
const activeNav = ref('Dashboard');
const sites = ref<Site[]>([]);
const sitesLoading = ref(false);
const discoveryOpen = ref(false);
const discoveryLoading = ref(false);
const discoveryError = ref('');
const discoveryPhase = ref('Ready to scan');
const discoveredSites = ref<DiscoveredSite[]>([]);
const selectedDiscoveredPaths = ref<string[]>([]);
const importMessage = ref('');
const webStationState = ref<'checking' | 'available' | 'unavailable'>('checking');

const navItems = computed(() => [
  { label: 'Dashboard', icon: LayoutDashboard },
  { label: 'Sites', icon: Globe2, count: sites.value.length || undefined },
  { label: 'Deployments', icon: UploadCloud, count: 0 },
  { label: 'Releases', icon: PackageCheck },
  { label: 'Activity', icon: Activity },
]);

const systemItems = [
  { label: 'Web Station', detail: 'Discovery ready', icon: Globe2, state: 'ready' },
  { label: 'Git transport', detail: 'SSH verification on', icon: GitBranch, state: 'ready' },
  { label: 'Release worker', detail: 'Idle · queue clear', icon: Zap, state: 'ready' },
  { label: 'SQLite', detail: 'Foundation migrated', icon: Database, state: 'ready' },
];

const pipelineSteps = [
  { label: 'Checkout', detail: 'main · a9f72cd', duration: '1.1s', status: 'done' },
  { label: 'Composer install', detail: 'No changes', duration: '7.4s', status: 'done' },
  { label: 'Frontend build', detail: 'vite build', duration: '8.8s', status: 'done' },
  { label: 'Health check', detail: '/up · HTTP 200', duration: '2.3s', status: 'done' },
];

const commands = [
  { label: 'Deploy servazar.ro', hint: '⌘ D', icon: Play },
  { label: 'Discover Web Station', hint: '⌘ W', icon: Globe2 },
  { label: 'Add a new site', hint: '⌘ N', icon: Plus },
  { label: 'Open settings', hint: '⌘ ,', icon: Settings2 },
];

const filteredCommands = computed(() => {
  const query = commandQuery.value.trim().toLowerCase();
  if (!query) return commands;
  return commands.filter((command) => command.label.toLowerCase().includes(query));
});

const healthLabel = computed(() => {
  if (healthState.value === 'healthy') return 'System healthy';
  if (healthState.value === 'offline') return 'API unavailable';
  return 'Checking system';
});

const webStationLabel = computed(() => {
  if (webStationState.value === 'available') return 'Read-only discovery available';
  if (webStationState.value === 'unavailable') return 'No configured roots detected';
  return 'Checking discovery roots';
});

function toggleTheme() {
  isDark.value = !isDark.value;
  document.documentElement.dataset.theme = isDark.value ? 'dark' : 'light';
}

function openCommandPalette() {
  commandOpen.value = true;
  commandQuery.value = '';
}

function closeCommandPalette() {
  commandOpen.value = false;
}

function runCommand(label: string) {
  closeCommandPalette();
  if (label === 'Discover Web Station') openDiscovery();
}

function siteClass(site: Site) {
  if (site.framework === 'wordpress') return 'site-blue';
  if (site.framework === 'flarum' || site.framework === 'symfony') return 'site-violet';
  return 'site-orange';
}

function displayStatus(status: string) {
  if (status === 'permission_required') return 'Permission required';
  if (status === 'active') return 'Ready';
  return status.replace(/_/g, ' ');
}

function onKeydown(event: KeyboardEvent) {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault();
    openCommandPalette();
  }
  if (event.key === 'Escape') closeCommandPalette();
}

async function checkHealth() {
  try {
    const response = await fetch('/releasestation/api/v1/system/health');
    healthState.value = response.ok ? 'healthy' : 'offline';
  } catch {
    healthState.value = 'offline';
  }
}

async function loadSites() {
  sitesLoading.value = true;
  try {
    const response = await fetch('/releasestation/api/v1/sites', { headers: { Accept: 'application/json' } });
    if (!response.ok) throw new Error('sites');
    const payload = await response.json();
    sites.value = payload.data || [];
  } catch {
    sites.value = [];
  } finally {
    sitesLoading.value = false;
  }
}

async function loadWebStationStatus() {
  try {
    const response = await fetch('/releasestation/api/v1/webstation/status', { headers: { Accept: 'application/json' } });
    if (!response.ok) throw new Error('webstation');
    const payload = await response.json();
    webStationState.value = payload.data?.available ? 'available' : 'unavailable';
  } catch {
    webStationState.value = 'unavailable';
  }
}

async function archiveSite(site: Site) {
  if (!window.confirm(`Archive ${site.hostname || site.name}?`)) return;
  const response = await fetch(`/releasestation/api/v1/sites/${site.id}`, { method: 'DELETE' });
  if (response.ok) await loadSites();
}

async function openDiscovery() {
  discoveryOpen.value = true;
  discoveryLoading.value = true;
  discoveryError.value = '';
  importMessage.value = '';
  selectedDiscoveredPaths.value = [];
  discoveredSites.value = [];
  discoveryPhase.value = 'Resolving Web Station roots';
  try {
    discoveryPhase.value = 'Detecting hosted applications';
    const response = await fetch('/releasestation/api/v1/webstation/discover', { method: 'POST', headers: { Accept: 'application/json' } });
    if (!response.ok) throw new Error('discovery');
    const payload = await response.json();
    discoveredSites.value = payload.data || [];
    selectedDiscoveredPaths.value = discoveredSites.value.filter((site) => !site.already_managed).map((site) => site.project_root);
    discoveryPhase.value = discoveredSites.value.length ? 'Review discovered applications' : 'No applications found';
  } catch {
    discoveryError.value = 'Web Station discovery is unavailable. Verify the configured read-only roots.';
    discoveryPhase.value = 'Discovery failed';
  } finally {
    discoveryLoading.value = false;
  }
}

async function importSelectedSites() {
  if (!selectedDiscoveredPaths.value.length) return;
  discoveryLoading.value = true;
  discoveryError.value = '';
  importMessage.value = '';
  try {
    const response = await fetch('/releasestation/api/v1/webstation/import', {
      method: 'POST',
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: JSON.stringify({ paths: selectedDiscoveredPaths.value }),
    });
    if (!response.ok) throw new Error('import');
    const payload = await response.json();
    const imported = payload.data?.imported || [];
    const skipped = payload.data?.skipped || [];
    importMessage.value = `${imported.length} site${imported.length === 1 ? '' : 's'} imported${skipped.length ? `, ${skipped.length} already managed` : ''}.`;
    await loadSites();
    selectedDiscoveredPaths.value = [];
    discoveredSites.value = discoveredSites.value.map((site) => ({ ...site, already_managed: imported.some((item: Site) => item.project_root === site.project_root) || site.already_managed }));
  } catch {
    discoveryError.value = 'Import failed. Check the site permissions and try again.';
  } finally {
    discoveryLoading.value = false;
  }
}

onMounted(() => {
  document.addEventListener('keydown', onKeydown);
  checkHealth();
  loadSites();
  loadWebStationStatus();
});

onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown));
</script>

<template>
  <div class="app-frame">
    <aside class="sidebar">
      <div class="brand-lockup">
        <div class="brand-mark"><Sparkles :size="18" /></div>
        <div>
          <div class="brand-name">Zion</div>
          <div class="brand-product">ReleaseStation</div>
        </div>
      </div>

      <button class="workspace-switcher" type="button">
        <span class="workspace-avatar">Z</span>
        <span class="workspace-copy"><strong>Development NAS</strong><small>DS1019+ · Apollo Lake</small></span>
        <ChevronDown :size="15" />
      </button>

      <div class="sidebar-section-label">Workspace</div>
      <nav class="primary-nav" aria-label="Primary navigation">
        <button v-for="item in navItems" :key="item.label" class="nav-item" :class="{ active: activeNav === item.label }" type="button" @click="activeNav = item.label">
          <component :is="item.icon" :size="17" />
          <span>{{ item.label }}</span>
          <span v-if="item.count" class="nav-count">{{ item.count }}</span>
        </button>
      </nav>

      <div class="sidebar-section-label sidebar-section-spaced">Manage</div>
      <nav class="primary-nav" aria-label="Management navigation">
        <button class="nav-item" type="button"><TerminalSquare :size="17" /><span>Pipelines</span></button>
        <button class="nav-item" type="button"><Webhook :size="17" /><span>Webhooks</span></button>
        <button class="nav-item" type="button"><ShieldCheck :size="17" /><span>Secrets</span><span class="nav-dot" /></button>
      </nav>

      <div class="sidebar-spacer" />
      <div class="sidebar-bottom">
        <button class="nav-item" type="button"><Settings2 :size="17" /><span>Settings</span></button>
        <button class="nav-item" type="button"><LifeBuoy :size="17" /><span>Help center</span></button>
        <div class="profile-card">
          <div class="profile-avatar">MR</div>
          <div class="profile-copy"><strong>matrixn</strong><small>Administrator</small></div>
          <MoreHorizontal :size="16" class="muted-icon" />
        </div>
      </div>
    </aside>

    <main class="main-content">
      <header class="topbar">
        <div class="breadcrumb"><span>Workspace</span><span class="breadcrumb-slash">/</span><strong>{{ activeNav }}</strong></div>
        <div class="topbar-actions">
          <button class="command-button" type="button" @click="openCommandPalette"><Search :size="16" /><span>Search</span><kbd>⌘ K</kbd></button>
          <button class="icon-button" type="button" aria-label="Toggle theme" @click="toggleTheme"><Sun v-if="isDark" :size="17" /><Moon v-else :size="17" /></button>
          <button class="icon-button" type="button" aria-label="Help"><CircleHelp :size="17" /></button>
          <div class="topbar-avatar">MR</div>
        </div>
      </header>

      <div class="content-wrap">
        <section v-if="activeNav === 'Dashboard'" class="hero-row">
          <div>
            <div class="eyebrow"><span class="eyebrow-pulse" /> DEVELOPMENT CONTROL PLANE</div>
            <h1>Ship with confidence.</h1>
            <p class="hero-copy">Your Synology deployment surface is clear, observable, and ready for the next release.</p>
          </div>
          <div class="hero-actions">
            <button class="button button-secondary" type="button" @click="openDiscovery"><Globe2 :size="16" />Discover Web Station</button>
            <button class="button button-primary" type="button"><Plus :size="17" />New project</button>
          </div>
        </section>

        <section v-if="activeNav === 'Dashboard'" class="metric-grid" aria-label="Workspace metrics">
          <article class="metric-card metric-card-accent">
            <div class="metric-top"><span class="metric-label">Managed sites</span><span class="metric-icon"><Globe2 :size="16" /></span></div>
            <div class="metric-value">{{ sites.length }} <span class="metric-muted">/ 5</span></div>
            <div class="metric-foot"><span class="trend-positive"><ArrowUpRight :size="14" />1 this week</span><span>Pro capacity</span></div>
          </article>
          <article class="metric-card">
            <div class="metric-top"><span class="metric-label">Successful deploys</span><span class="metric-icon green"><Check :size="16" /></span></div>
            <div class="metric-value">98.4<span class="metric-unit">%</span></div>
            <div class="metric-foot"><span class="trend-positive"><ArrowUpRight :size="14" />2.8%</span><span>Last 30 days</span></div>
          </article>
          <article class="metric-card">
            <div class="metric-top"><span class="metric-label">Median deploy time</span><span class="metric-icon violet"><Clock3 :size="16" /></span></div>
            <div class="metric-value">31<span class="metric-unit">.2s</span></div>
            <div class="metric-foot"><span class="trend-positive"><ArrowDownToLine :size="14" />4.1s faster</span><span>Last 30 days</span></div>
          </article>
          <article class="metric-card">
            <div class="metric-top"><span class="metric-label">Queue status</span><span class="metric-icon blue"><ListChecks :size="16" /></span></div>
            <div class="metric-value">Clear</div>
            <div class="metric-foot"><span class="status-inline"><span class="status-dot" />Worker ready</span><span>2 concurrency</span></div>
          </article>
        </section>

        <section v-if="activeNav === 'Dashboard'" class="dashboard-grid">
          <article class="panel pipeline-panel">
            <div class="panel-heading">
              <div><div class="panel-kicker"><span class="live-dot" /> LIVE DEPLOYMENT</div><h2>servazar.ro</h2></div>
              <button class="more-button" type="button"><MoreHorizontal :size="18" /></button>
            </div>
            <div class="deploy-meta"><span class="branch-chip"><GitBranch :size="14" />main</span><code>a9f72cd</code><span class="meta-separator">·</span><span>just now</span><span class="deploy-state"><span class="status-dot" />Healthy</span></div>
            <div class="pipeline-track">
              <div v-for="step in pipelineSteps" :key="step.label" class="pipeline-step">
                <div class="step-marker"><Check :size="13" /></div>
                <div class="step-copy"><strong>{{ step.label }}</strong><span>{{ step.detail }}</span></div>
                <code>{{ step.duration }}</code>
              </div>
            </div>
            <div class="terminal-preview">
              <div class="terminal-header"><span><span class="terminal-dot red" /><span class="terminal-dot yellow" /><span class="terminal-dot green" /></span><span>release #184 · output</span><button type="button">Open logs <ArrowUpRight :size="13" /></button></div>
              <div class="terminal-body"><p><span class="terminal-muted">$</span> php artisan optimize</p><p class="terminal-success"><span>✓</span> Configuration cached successfully.</p><p><span class="terminal-muted">$</span> curl -fsS https://servazar.ro/up</p><p class="terminal-success"><span>✓</span> HTTP 200 · deployment active <span class="cursor" /></p></div>
            </div>
          </article>

          <article class="panel activity-panel">
            <div class="panel-heading"><div><div class="panel-kicker">SYSTEM OVERVIEW</div><h2>Everything is ready</h2></div><span class="health-badge" :class="healthState"><span class="status-dot" />{{ healthLabel }}</span></div>
            <div class="system-list">
              <div v-for="item in systemItems" :key="item.label" class="system-row"><span class="system-icon"><component :is="item.icon" :size="15" /></span><span class="system-copy"><strong>{{ item.label }}</strong><small>{{ item.label === 'Web Station' ? webStationLabel : item.detail }}</small></span><Check :size="16" class="system-check" /></div>
            </div>
            <div class="system-foot"><span>DSM 7.4-90075</span><span class="system-foot-separator">·</span><span>x86_64 / apollolake</span><button type="button" @click="checkHealth"><RotateCw :size="14" />Refresh</button></div>
          </article>
        </section>

        <section v-if="activeNav === 'Dashboard'" class="section-heading"><div><div class="panel-kicker">YOUR SURFACE</div><h2>Managed sites</h2></div><button class="text-button" type="button" @click="activeNav = 'Sites'">View all sites <ArrowUpRight :size="15" /></button></section>
        <section v-if="activeNav === 'Dashboard'" class="sites-grid">
          <article v-if="sites.length === 0" class="site-card empty-site-card">
            <Globe2 :size="20" />
            <strong>{{ sitesLoading ? 'Loading managed sites' : 'No managed sites yet' }}</strong>
            <span>Discover existing Web Station applications or add a site manually.</span>
          </article>
          <article v-for="site in sites" :key="site.id" class="site-card" :class="siteClass(site)">
            <div class="site-card-top"><span class="framework-mark"><Code2 :size="17" /></span><button class="more-button" type="button"><MoreHorizontal :size="17" /></button></div>
            <div class="site-domain">{{ site.hostname || site.name }}</div><div class="site-framework">{{ site.framework }}</div>
            <div class="site-status" :class="{ 'site-status-warning': site.status !== 'active' }"><span class="status-dot" />{{ displayStatus(site.status) }}<span class="site-status-time">{{ site.strategy }}</span></div>
            <div class="site-card-foot"><span><Globe2 :size="14" />{{ site.web_root }}</span><code>{{ site.project_root }}</code><button type="button" aria-label="Deploy site" disabled><Play :size="14" /></button></div>
          </article>
          <button class="site-card add-site-card" type="button" @click="openDiscovery"><span class="add-site-icon"><Plus :size="19" /></span><strong>Discover Web Station</strong><span>Import existing applications into ReleaseStation</span></button>
        </section>

        <section v-if="activeNav === 'Sites'" class="sites-management">
          <div class="sites-management-header"><div><div class="eyebrow"><span class="eyebrow-pulse" /> SITE CATALOG</div><h1>Managed sites.</h1><p class="hero-copy">Review imported Web Station applications, document roots and deployment readiness.</p></div><div class="hero-actions"><button class="button button-secondary" type="button" @click="loadSites"><RotateCw :size="15" />Refresh</button><button class="button button-primary" type="button" @click="openDiscovery"><Globe2 :size="15" />Discover Web Station</button></div></div>
          <div class="sites-management-summary"><span><strong>{{ sites.length }}</strong> managed sites</span><span><span class="status-dot" />{{ webStationLabel }}</span><span>Discovery adapter: read-only</span></div>
          <div v-if="sites.length" class="management-list">
            <article v-for="site in sites" :key="site.id" class="management-row">
              <span class="framework-mark"><Code2 :size="17" /></span><span class="management-copy"><strong>{{ site.hostname || site.name }}</strong><small>{{ site.framework }} · {{ site.web_root }}</small><small>{{ site.project_root }}</small></span><span class="discovery-badge" :class="site.status === 'active' ? 'ready' : 'read_only'">{{ displayStatus(site.status) }}</span><button class="more-button" type="button" aria-label="Archive site" @click="archiveSite(site)"><X :size="15" /></button>
            </article>
          </div>
          <div v-else class="management-empty"><Globe2 :size="24" /><strong>No sites are managed yet</strong><span>Start with a read-only discovery scan of Web Station.</span><button class="button button-primary" type="button" @click="openDiscovery">Discover applications</button></div>
        </section>

        <footer v-if="activeNav === 'Dashboard'" class="footer-note"><span><ShieldCheck :size="14" />Protected by ReleaseStation guardrails</span><span>v0.1.0 · Foundation milestone</span></footer>
      </div>
    </main>

    <Transition name="palette-fade">
      <div v-if="commandOpen" class="palette-backdrop" @click.self="closeCommandPalette">
        <div class="command-palette" role="dialog" aria-modal="true" aria-label="Command palette">
          <div class="palette-search"><Search :size="18" /><input v-model="commandQuery" autofocus placeholder="Search or run a command..." /><kbd>ESC</kbd><button type="button" aria-label="Close command palette" @click="closeCommandPalette"><X :size="17" /></button></div>
          <div class="palette-label">Quick actions</div>
          <button v-for="command in filteredCommands" :key="command.label" class="palette-command" type="button" @click="runCommand(command.label)"><span class="palette-command-icon"><component :is="command.icon" :size="16" /></span><span>{{ command.label }}</span><kbd>{{ command.hint }}</kbd></button>
          <div v-if="filteredCommands.length === 0" class="palette-empty">No commands match “{{ commandQuery }}”.</div>
          <div class="palette-footer"><span><Command :size="13" /> Navigate</span><span><ArrowUpRight :size="13" /> Open</span><span><CircleHelp :size="13" /> Help</span></div>
        </div>
      </div>
    </Transition>

    <Transition name="palette-fade">
      <div v-if="discoveryOpen" class="palette-backdrop" @click.self="discoveryOpen = false">
        <section class="discovery-dialog" role="dialog" aria-modal="true" aria-labelledby="discovery-title">
          <header class="discovery-header">
            <div><div class="panel-kicker"><Globe2 :size="13" /> WEB STATION DISCOVERY</div><h2 id="discovery-title">Import hosted applications</h2><p>Read-only scan of configured Web Station roots. Nothing in Web Station is changed.</p></div>
            <button class="icon-button" type="button" aria-label="Close discovery" @click="discoveryOpen = false"><X :size="17" /></button>
          </header>
          <div class="discovery-phase"><span class="status-dot" :class="{ 'discovery-pulse': discoveryLoading }" />{{ discoveryPhase }}</div>
          <div v-if="discoveryError" class="discovery-error"><CircleAlert :size="16" />{{ discoveryError }}</div>
          <div v-if="importMessage" class="discovery-success"><Check :size="16" />{{ importMessage }}</div>
          <div v-if="!discoveryLoading && discoveredSites.length === 0 && !discoveryError" class="discovery-empty"><Globe2 :size="22" /><strong>No applications discovered</strong><span>Check that Web Station document roots exist under the configured read-only roots.</span></div>
          <div v-else class="discovery-list">
            <label v-for="site in discoveredSites" :key="site.project_root" class="discovery-row" :class="{ managed: site.already_managed }">
              <input v-model="selectedDiscoveredPaths" type="checkbox" :value="site.project_root" :disabled="site.already_managed">
              <span class="discovery-copy"><strong>{{ site.hostname || site.name }}</strong><small>{{ site.framework }} · {{ site.web_root }}</small><small>{{ site.permissions.message }}</small></span>
              <span class="discovery-badge" :class="site.permissions.status">{{ site.already_managed ? 'Managed' : site.permissions.status }}</span>
            </label>
          </div>
          <footer class="discovery-footer"><span>{{ selectedDiscoveredPaths.length }} selected</span><div><button class="button button-secondary" type="button" @click="openDiscovery">Rescan</button><button class="button button-primary" type="button" :disabled="discoveryLoading || !selectedDiscoveredPaths.length" @click="importSelectedSites">Import selected</button></div></footer>
        </section>
      </div>
    </Transition>
  </div>
</template>
