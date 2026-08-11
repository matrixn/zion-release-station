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

const healthState = ref<HealthState>('checking');
const isDark = ref(true);
const commandOpen = ref(false);
const commandQuery = ref('');
const activeNav = ref('Dashboard');

const navItems = [
  { label: 'Dashboard', icon: LayoutDashboard },
  { label: 'Sites', icon: Globe2, count: 4 },
  { label: 'Deployments', icon: UploadCloud, count: 12 },
  { label: 'Releases', icon: PackageCheck },
  { label: 'Activity', icon: Activity },
];

const systemItems = [
  { label: 'Web Station', detail: 'Discovery ready', icon: Globe2, state: 'ready' },
  { label: 'Git transport', detail: 'SSH verification on', icon: GitBranch, state: 'ready' },
  { label: 'Release worker', detail: 'Idle · queue clear', icon: Zap, state: 'ready' },
  { label: 'SQLite', detail: 'Foundation migrated', icon: Database, state: 'ready' },
];

const sites = [
  { domain: 'servazar.ro', framework: 'Laravel', branch: 'main', commit: 'a9f72cd', time: '4 min ago', color: 'orange', status: 'Healthy' },
  { domain: 'zion3d.ro', framework: 'WordPress', branch: 'production', commit: '8d3b2a1', time: '28 min ago', color: 'blue', status: 'Healthy' },
  { domain: 'support.zion3d.ro', framework: 'Flarum', branch: 'main', commit: 'f42e901', time: '1 hr ago', color: 'violet', status: 'Healthy' },
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

onMounted(() => {
  document.addEventListener('keydown', onKeydown);
  checkHealth();
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
        <section class="hero-row">
          <div>
            <div class="eyebrow"><span class="eyebrow-pulse" /> DEVELOPMENT CONTROL PLANE</div>
            <h1>Ship with confidence.</h1>
            <p class="hero-copy">Your Synology deployment surface is clear, observable, and ready for the next release.</p>
          </div>
          <div class="hero-actions">
            <button class="button button-secondary" type="button" @click="openCommandPalette"><Globe2 :size="16" />Discover Web Station</button>
            <button class="button button-primary" type="button"><Plus :size="17" />New project</button>
          </div>
        </section>

        <section class="metric-grid" aria-label="Workspace metrics">
          <article class="metric-card metric-card-accent">
            <div class="metric-top"><span class="metric-label">Managed sites</span><span class="metric-icon"><Globe2 :size="16" /></span></div>
            <div class="metric-value">4 <span class="metric-muted">/ 5</span></div>
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

        <section class="dashboard-grid">
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
              <div v-for="item in systemItems" :key="item.label" class="system-row"><span class="system-icon"><component :is="item.icon" :size="15" /></span><span class="system-copy"><strong>{{ item.label }}</strong><small>{{ item.detail }}</small></span><Check :size="16" class="system-check" /></div>
            </div>
            <div class="system-foot"><span>DSM 7.4-90075</span><span class="system-foot-separator">·</span><span>x86_64 / apollolake</span><button type="button" @click="checkHealth"><RotateCw :size="14" />Refresh</button></div>
          </article>
        </section>

        <section class="section-heading"><div><div class="panel-kicker">YOUR SURFACE</div><h2>Managed sites</h2></div><button class="text-button" type="button">View all sites <ArrowUpRight :size="15" /></button></section>
        <section class="sites-grid">
          <article v-for="site in sites" :key="site.domain" class="site-card" :class="`site-${site.color}`">
            <div class="site-card-top"><span class="framework-mark"><Code2 :size="17" /></span><button class="more-button" type="button"><MoreHorizontal :size="17" /></button></div>
            <div class="site-domain">{{ site.domain }}</div><div class="site-framework">{{ site.framework }}</div>
            <div class="site-status"><span class="status-dot" />{{ site.status }}<span class="site-status-time">{{ site.time }}</span></div>
            <div class="site-card-foot"><span><GitBranch :size="14" />{{ site.branch }}</span><code>{{ site.commit }}</code><button type="button" aria-label="Deploy site"><Play :size="14" /></button></div>
          </article>
          <button class="site-card add-site-card" type="button" @click="openCommandPalette"><span class="add-site-icon"><Plus :size="19" /></span><strong>Add a new site</strong><span>Import from Web Station or configure manually</span></button>
        </section>

        <footer class="footer-note"><span><ShieldCheck :size="14" />Protected by ReleaseStation guardrails</span><span>v0.1.0 · Foundation milestone</span></footer>
      </div>
    </main>

    <Transition name="palette-fade">
      <div v-if="commandOpen" class="palette-backdrop" @click.self="closeCommandPalette">
        <div class="command-palette" role="dialog" aria-modal="true" aria-label="Command palette">
          <div class="palette-search"><Search :size="18" /><input v-model="commandQuery" autofocus placeholder="Search or run a command..." /><kbd>ESC</kbd><button type="button" aria-label="Close command palette" @click="closeCommandPalette"><X :size="17" /></button></div>
          <div class="palette-label">Quick actions</div>
          <button v-for="command in filteredCommands" :key="command.label" class="palette-command" type="button" @click="closeCommandPalette"><span class="palette-command-icon"><component :is="command.icon" :size="16" /></span><span>{{ command.label }}</span><kbd>{{ command.hint }}</kbd></button>
          <div v-if="filteredCommands.length === 0" class="palette-empty">No commands match “{{ commandQuery }}”.</div>
          <div class="palette-footer"><span><Command :size="13" /> Navigate</span><span><ArrowUpRight :size="13" /> Open</span><span><CircleHelp :size="13" /> Help</span></div>
        </div>
      </div>
    </Transition>
  </div>
</template>
