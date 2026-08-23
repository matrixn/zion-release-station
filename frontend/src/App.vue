<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { basicSetup, EditorView } from 'codemirror';
import { StreamLanguage } from '@codemirror/language';
import { shell } from '@codemirror/legacy-modes/mode/shell';
import { oneDark } from '@codemirror/theme-one-dark';
import { EditorState } from '@codemirror/state';
import {
  Activity,
  ArrowDownToLine,
  ArrowUpRight,
  Box,
  Check,
  CircleAlert,
  CircleDot,
  CircleX,
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
  custom_framework?: string;
  strategy: string;
  status: string;
  tags?: string[];
  color?: string;
  push_to_deploy?: boolean;
  deploy_script?: string;
  deployment_retention?: number;
  health_check_url?: string;
  shared_directories?: string[];
  runtime?: Record<string, any>;
  created_at: string;
  updated_at: string;
  repository?: { provider: string; clone_url: string; branch: string; github_installation_id?: number; github_repository_id?: number; github_full_name?: string; github_default_branch?: string };
};
type DiscoveredSite = Site & {
  detection: { confidence: string; evidence: string[]; document_root: string };
  permissions: { status: string; readable: boolean; writable: boolean; deployable: boolean; message: string };
  source: string;
  already_managed: boolean;
};
type GithubRepository = {
  installation_id: number;
  account_login: string;
  id: number;
  name: string;
  full_name: string;
  private: boolean;
  default_branch: string;
  clone_url: string;
  ssh_url: string;
};
type GithubInstallation = { github_installation_id: number; account_login: string; account_type: string; repository_selection: string; };
type Deployment = { id: string; site_id: string; trigger_type: string; trigger_reference?: string; deployment_method?: string; branch: string; commit_sha: string; commit_message: string; commit_url: string; status: string; error_code?: string; error_summary?: string; queued_at: string; started_at?: string; finished_at?: string; duration_ms?: number; created_at: string; build_log?: string; deployment_log?: string; steps?: { id: string; step_key: string; name: string; status: string; duration_ms?: number }[]; };
type Release = { id: string; site_id: string; deployment_id: string; release_name: string; release_path: string; commit_sha?: string; commit_message?: string; commit_url?: string; branch?: string; active: boolean; health_status?: string; created_at: string; activated_at?: string; };
type SiteWebhook = { id?: string; site_id: string; provider: string; enabled: boolean; configured: boolean; secret_configured: boolean; endpoint?: string; last_delivery_at?: string; last_error?: string };
type Commit = { sha: string; message: string; branch: string; author?: string; url?: string; created_at?: string; deployed: boolean; included_in_deployed?: boolean; deployment_id?: string; status: string; };

type GithubState = {
  configured: boolean;
  mode: 'managed' | 'self_hosted' | string;
  configuration_error: string;
  connected: boolean;
  app_slug: string;
  setup_url: string;
  account_login?: string;
  installations: GithubInstallation[];
  webhook_configured?: boolean;
  webhook_endpoint?: string;
  webhook_accepted_events?: number;
  webhook_last_event_at?: string;
};

const healthState = ref<HealthState>('checking');
const themeStorageKey = 'zion-releasestation-theme';
const packageIconURL = '/webman/3rdparty/zion-releasestation/images/app_64.png';
function readStoredTheme() {
  try {
    return localStorage.getItem(themeStorageKey) === 'light' ? false : true;
  } catch {
    return true;
  }
}
const isDark = ref(readStoredTheme());
const commandOpen = ref(false);
const commandQuery = ref('');
const navigationStorageKey = 'zion-releasestation-navigation';
function readStoredNavigation() {
  try {
    const value = localStorage.getItem(navigationStorageKey);
    return ['Dashboard', 'Sites', 'Activity', 'Settings', 'Help'].includes(value || '') ? value || 'Dashboard' : 'Dashboard';
  } catch {
    return 'Dashboard';
  }
}
const activeNav = ref(readStoredNavigation());
const selectedSite = ref<Site | null>(null);
const selectedSiteTab = ref<'Overview' | 'Repository' | 'Deployments' | 'Settings'>('Overview');
const selectedDeployment = ref<Deployment | null>(null);
const siteDeployments = ref<Deployment[]>([]);
const siteCommits = ref<Commit[]>([]);
const siteReleases = ref<Release[]>([]);
const releasesLoading = ref(false);
const rollbackReleaseID = ref('');
const releaseMessage = ref('');
const releaseError = ref('');
const sitePage = ref(1);
const siteTotalPages = ref(1);
const deploymentSearch = ref('');
const siteDetailLoading = ref(false);
const deploymentDetailLoading = ref(false);
const deployingCommitSha = ref('');
const logsExpanded = ref(true);
const sites = ref<Site[]>([]);
const sitesLoading = ref(false);
const discoveryOpen = ref(false);
const discoveryLoading = ref(false);
const discoveryError = ref('');
const discoveryPhase = ref('Ready to scan');
const discoveredSites = ref<DiscoveredSite[]>([]);
const selectedDiscoveredPaths = ref<string[]>([]);
const importMessage = ref('');
const importClosing = ref(false);
const importCountdown = ref(5);
const discoveryDialog = ref<HTMLElement | null>(null);
const webStationState = ref<'checking' | 'available' | 'unavailable'>('checking');
const wizardOpen = ref(false);
const wizardStep = ref(1);
const wizardSaving = ref(false);
const wizardError = ref('');
const wizardForm = ref({
  name: '',
  url: '',
  projectRoot: '',
  webRoot: '',
  framework: 'auto',
  provider: 'github',
  cloneUrl: '',
  branch: 'main',
  strategy: 'in_place',
  githubInstallationId: null as number | null,
  githubRepositoryId: null as number | null,
  githubFullName: '',
  githubDefaultBranch: '',
});
const githubState = ref<GithubState>({ configured: false, mode: 'managed', configuration_error: '', connected: false, app_slug: '', setup_url: '', installations: [], webhook_configured: false, webhook_endpoint: '', webhook_accepted_events: 0, webhook_last_event_at: '' });
const githubRepositories = ref<GithubRepository[]>([]);
const githubBranches = ref<string[]>([]);
const githubBranchesLoading = ref(false);
const githubBranchesError = ref('');
const githubLoading = ref(false);
const githubAccount = ref('');
const githubSaving = ref(false);
const githubMessage = ref('');
const githubError = ref('');
const githubConnectState = ref<'idle' | 'starting' | 'waiting'>('idle');
const deployingSiteId = ref('');
const deployMessage = ref('');
const deployError = ref('');
const repositorySaving = ref(false);
const repositoryMessage = ref('');
const repositoryError = ref('');
const gitTransportState = ref<'idle' | 'testing' | 'success' | 'error'>('idle');
const gitTransportMessage = ref('');
const deployPublicKey = ref('');
const deployKeyFingerprint = ref('');
const deployKeyLoading = ref(false);
const repositoryForm = ref({
  provider: 'github',
  clone_url: '',
  branch: 'main',
  github_installation_id: null as number | null,
  github_repository_id: null as number | null,
  github_full_name: '',
  github_default_branch: '',
  strategy: 'in_place',
});
const siteSettingsSaving = ref(false);
const siteSettingsMessage = ref('');
const siteSettingsError = ref('');
const siteWebhook = ref<SiteWebhook>({ site_id: '', provider: 'github', enabled: false, configured: false, secret_configured: false });
const siteWebhookLoading = ref(false);
const siteWebhookRotating = ref(false);
const siteWebhookSecret = ref('');
const siteWebhookMessage = ref('');
const siteWebhookError = ref('');
const siteTagDraft = ref('');
const siteSettingsForm = ref({
  framework: 'other',
  custom_framework: '',
  tags: [] as string[],
  color: '#f28c3b',
  push_to_deploy: false,
  deploy_script: '',
  deployment_retention: 4,
  health_check_url: '',
  shared_directories: [] as string[],
});
const sharedDirectoryDraft = ref('');
const deployScriptEditor = ref<HTMLElement | null>(null);
let deployScriptEditorView: EditorView | null = null;
type DashboardMetrics = {
  successful_deploys: number;
  total_deploys: number;
  median_duration_ms: number;
  running_deploys: number;
  queued_deploys: number;
  queue_status: string;
  latest?: { deployment_id: string; site_id: string; site_name: string; status: string; branch: string; commit_sha: string; commit_message: string; created_at: string; duration_ms: number };
  services: { id: string; label: string; state: string; detail: string; command?: string; description?: string; install_hint?: string; version?: string }[];
};
type AuditEntry = { id: string; actor_type: string; actor_id?: string; action: string; entity_type?: string; entity_id?: string; metadata?: Record<string, any>; created_at: string };
const dashboardMetrics = ref<DashboardMetrics>({ successful_deploys: 0, total_deploys: 0, median_duration_ms: 0, running_deploys: 0, queued_deploys: 0, queue_status: 'idle', services: [] });
const auditEntries = ref<AuditEntry[]>([]);
const auditLoading = ref(false);
const deploymentNotice = ref<{ id: string; siteID: string; siteName: string; status: string; progress: number; message: string } | null>(null);
type SystemCheckSetting = { id: string; label: string; command: string; description: string; install_hint: string; enabled: boolean };
const systemChecks = ref<SystemCheckSetting[]>([]);
const systemChecksLoading = ref(false);
const systemChecksSaving = ref(false);
const systemChecksMessage = ref('');
const systemChecksError = ref('');
watch(activeNav, (value) => {
  try { localStorage.setItem(navigationStorageKey, value); } catch { /* localStorage is optional */ }
});
watch(selectedSiteTab, (value) => {
  if (value === 'Settings') nextTick(mountDeployScriptEditor);
  else destroyDeployScriptEditor();
});
let githubPollTimer: ReturnType<typeof setInterval> | undefined;
let importCloseTimer: ReturnType<typeof setTimeout> | undefined;
let dashboardRefreshTimer: ReturnType<typeof setInterval> | undefined;
let deploymentToastTimer: ReturnType<typeof setTimeout> | undefined;
let deploymentEventSource: EventSource | undefined;
let deploymentNoticeTimer: ReturnType<typeof setTimeout> | undefined;
let githubPairingSessionId = '';
let githubPairingToken = '';
let githubPairingHandled = false;

function frameworkLabel(framework: string) {
  const labels: Record<string, string> = { laravel: 'Laravel', wordpress: 'WordPress', php: 'PHP', static: 'Plain script', flarum: 'Flarum' };
  return labels[framework] || (framework ? framework.charAt(0).toUpperCase() + framework.slice(1) : 'Plain script');
}

function relativeTime(value?: string) {
  if (!value) return 'Not yet';
  const seconds = Math.max(0, Math.floor((Date.now() - new Date(value).getTime()) / 1000));
  if (seconds < 60) return 'just now';
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  return `${Math.floor(seconds / 86400)}d ago`;
}

function durationLabel(milliseconds?: number) {
  if (!milliseconds) return '—';
  if (milliseconds < 1000) return `${milliseconds}ms`;
  return `${(milliseconds / 1000).toFixed(1)}s`;
}

function createdLabel(value?: string) {
  if (!value) return 'Unknown';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'Unknown';
  const parts = new Intl.DateTimeFormat('ro-RO', { day: 'numeric', month: 'long', year: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false }).formatToParts(date);
  const part = (type: string) => parts.find((item) => item.type === type)?.value || '';
  const month = part('month');
  return `${part('day')} ${month.charAt(0).toUpperCase()}${month.slice(1)} ${part('year')} ${part('hour')}:${part('minute')}`;
}

function siteRuntime(site: Site | null, key: string, fallback = 'Not detected') {
  const runtime = site?.runtime || {};
  const metadata = (runtime.metadata || {}) as Record<string, any>;
  return String(runtime[key] || metadata[key] || fallback);
}

function sitePublicURL(site: Site | null) {
  if (!site) return '';
  return siteRuntime(site, 'url', site.hostname ? `https://${site.hostname}` : 'Not configured');
}

function copyPublicIP(site: Site | null) {
  const value = siteRuntime(site, 'public_ip', 'Not detected');
  if (value !== 'Not detected') navigator.clipboard?.writeText(value);
}

function emptyGithubState(): GithubState {
  return { configured: false, mode: 'managed', configuration_error: 'Synology Connector is not provisioned for this Release Station instance', connected: false, app_slug: '', setup_url: '', installations: [], webhook_configured: false, webhook_endpoint: '', webhook_accepted_events: 0, webhook_last_event_at: '' };
}

const navItems = computed(() => [
  { label: 'Dashboard', icon: LayoutDashboard },
  { label: 'Sites', icon: Globe2, count: sites.value.length || undefined },
  { label: 'Activity', icon: Activity },
]);

const systemIcons: Record<string, any> = { webstation: Globe2, github_connector: GitBranch, sqlite: Database, php: Code2, composer: PackageCheck, node: ServerCog, npm: Hammer, git: Zap, rsync: ArrowDownToLine, unzip: Box, tar: PackageCheck, curl: Globe2, mysql: Database };
const systemItems = computed(() => dashboardMetrics.value.services.map((service) => ({ ...service, icon: systemIcons[service.id] || Activity })));
const helpTopic = ref('intro');
const helpArticles: Record<string, { title: string; summary: string; steps: string[] }> = {
  'Web Station': {
    title: 'Web Station discovery',
    summary: 'Release Station citește configurațiile Web Station și separă rădăcina proiectului de document root-ul servit public.',
    steps: ['Verifică în Settings că rădăcina Web Station este una dintre căile read-only configurate.', 'Acordă utilizatorului serviciului Release Station drept de citire pe /volume1/web sau pe rădăcina reală a site-urilor.', 'Dacă vezi permission denied, repară ACL-ul din DSM pentru utilizatorul serviciului și rulează din nou Discover Web Station.'],
  },
  'Git transport': {
    title: 'Git Transport',
    summary: 'Acest serviciu verifică faptul că NAS-ul poate folosi transportul Git necesar pentru repository-uri și deploy.',
    steps: ['Confirmă că binarul git există pe NAS și poate fi executat de serviciul pachetului.', 'Pentru repository-uri private, reconectează GitHub prin aplicația GitHub „Synology Connector” din Settings și aprobă repository-ul.', 'Dacă repository-ul este configurat manual, verifică URL-ul clone și branch-ul selectat.'],
  },
  'GitHub connector': {
    title: 'GitHub connector',
    summary: 'Synology Connector intermediază accesul la repository-urile GitHub, inclusiv private, fără ca cheia aplicației să fie stocată pe NAS.',
    steps: ['Deschide Settings și apasă Connect GitHub.', 'Autentifică-te în GitHub și instalează aplicația GitHub „Synology Connector” în contul sau organizația dorită.', 'Aprobă repository-urile, apoi revino în Release Station și apasă Refresh repositories.'],
  },
  SQLite: {
    title: 'SQLite database',
    summary: 'Release Station folosește SQLite local pentru site-uri, deployment-uri, release-uri și starea connectorului.',
    steps: ['Verifică în Package Health că serviciul local este healthy.', 'Nu șterge fișierul bazei de date din directorul pachetului.', 'Dacă baza este blocată, oprește și pornește pachetul din Package Center înainte de a repeta operația.'],
  },
  'Atomic releases': {
    title: 'Atomic releases and rollback',
    summary: 'Fiecare release este păstrat separat în .zion/releases, iar .current indică versiunea activă. După activare, URL-ul de health check decide dacă versiunea rămâne live.',
    steps: ['Configurează un Health check URL în site → Settings → Deployment. Fără URL, deployment-ul este marcat skipped și nu poate declanșa rollback automat.', 'Adaugă directoare persistente precum storage sau uploads la Shared directories. Ele sunt păstrate în .zion/shared și legate în fiecare release.', 'Pentru rollback, deschide site → Overview → Release history și alege Rollback here. Sunt restaurate fișierele aplicației; migrațiile bazei de date nu sunt inversate automat.'],
  },
};
function systemItem(topic: string) {
  return systemItems.value.find((item) => item.label === topic);
}

function helpSteps(topic: string) {
  const article = helpArticles[topic];
  if (article?.steps?.length) return article.steps;
  const item = systemItem(topic);
  if (!item) return ['Revino pe Dashboard și apasă Refresh pentru o verificare live.'];
  return [
    `Pe Synology, verifică mai întâi în Package Center dacă este instalat ${item.label}.`,
    item.install_hint || `Conectează-te prin SSH și verifică binarul cu: ${item.command} --version`,
    'Dacă instalarea este făcută, verifică PATH-ul utilizatorului serviciului și apasă Refresh în System Overview.',
  ];
}

function openHelp(topic = 'intro') {
  helpTopic.value = topic;
  activeNav.value = 'Help';
}

const featuredSite = computed(() => sites.value.find((site) => site.repository?.provider === 'github') || null);
const githubRepositoryGroups = computed(() => {
  const groups = new Map<string, GithubRepository[]>();
  for (const repository of githubRepositories.value) {
    const key = repository.account_login || repository.full_name.split('/')[0] || 'unknown';
    groups.set(key, [...(groups.get(key) || []), repository]);
  }
  return [...groups.entries()].sort(([a], [b]) => a.localeCompare(b)).map(([account, repositories]) => ({ account, repositories: repositories.sort((a, b) => a.name.localeCompare(b.name)) }));
});
const selectableDiscoveredPaths = computed(() => discoveredSites.value.filter((site) => !site.already_managed).map((site) => site.project_root));
const allDiscoverableSelected = computed(() => selectableDiscoveredPaths.value.length > 0 && selectableDiscoveredPaths.value.every((path) => selectedDiscoveredPaths.value.includes(path)));

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
  applyTheme();
}

function applyTheme() {
  const theme = isDark.value ? 'dark' : 'light';
  document.documentElement.dataset.theme = theme;
  try {
    localStorage.setItem(themeStorageKey, theme);
  } catch {
    // Storage can be disabled in a browser privacy mode; the current session still works.
  }
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
  if (label === 'Add a new site') openWizard();
  if (label === 'Open settings') activeNav.value = 'Settings';
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

async function loadDashboardMetrics() {
  try {
    const response = await fetch('/releasestation/api/v1/system/metrics', { headers: { Accept: 'application/json' } });
    if (!response.ok) throw new Error('metrics');
    const payload = await response.json();
    const next = payload.data || dashboardMetrics.value;
    const latest = next.latest;
    if (latest?.deployment_id && !deploymentTerminal(latest.status) && deploymentNotice.value?.id !== latest.deployment_id) {
      setDeploymentNotice(latest.deployment_id, latest.site_id, latest.site_name, latest.status, 'Deployment is running. Click to follow live logs.');
      watchDeploymentStream(latest.deployment_id, latest.site_id);
    }
    dashboardMetrics.value = next;
  } catch {
    // Keep the last known values visible when a single polling request fails.
  }
}

async function loadAuditLogs() {
  auditLoading.value = true;
  try {
    const response = await fetch('/releasestation/api/v1/audit-logs', { headers: { Accept: 'application/json' } });
    if (!response.ok) throw new Error('audit');
    const payload = await response.json();
    auditEntries.value = payload.data || [];
  } catch {
    // Audit remains read-only and non-blocking when the API is upgrading.
  } finally {
    auditLoading.value = false;
  }
}

async function loadSystemChecks() {
  systemChecksLoading.value = true;
  try {
    const response = await fetch('/releasestation/api/v1/system/checks', { headers: { Accept: 'application/json' } });
    if (!response.ok) throw new Error('checks');
    const payload = await response.json();
    systemChecks.value = payload.data?.checks || [];
  } catch {
    systemChecksError.value = 'Nu am putut încărca verificările configurabile.';
  } finally {
    systemChecksLoading.value = false;
  }
}

async function saveSystemChecks() {
  systemChecksSaving.value = true;
  systemChecksMessage.value = '';
  systemChecksError.value = '';
  try {
    const enabled = systemChecks.value.filter((check) => check.enabled).map((check) => check.id);
    const response = await fetch('/releasestation/api/v1/system/checks', { method: 'PUT', headers: { Accept: 'application/json', 'Content-Type': 'application/json' }, body: JSON.stringify({ enabled }) });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Nu am putut salva verificările.');
    await loadDashboardMetrics();
    systemChecksMessage.value = 'Verificările System Overview au fost salvate și reîmprospătate.';
  } catch (error) {
    systemChecksError.value = error instanceof Error ? error.message : 'Nu am putut salva verificările.';
  } finally {
    systemChecksSaving.value = false;
  }
}

function resetRepositoryForm(site: Site) {
  const repository = site.repository;
  repositoryForm.value = {
    provider: repository?.provider || 'github',
    clone_url: repository?.clone_url || '',
    branch: repository?.branch || repository?.github_default_branch || 'main',
    github_installation_id: repository?.github_installation_id || null,
    github_repository_id: repository?.github_repository_id || null,
    github_full_name: repository?.github_full_name || '',
    github_default_branch: repository?.github_default_branch || '',
    strategy: site.strategy === 'atomic' ? 'atomic' : 'in_place',
  };
}

function openRepositoryTab() {
  if (!selectedSite.value) return;
  resetRepositoryForm(selectedSite.value);
  repositoryMessage.value = '';
  repositoryError.value = '';
  selectedSiteTab.value = 'Repository';
  if (githubState.value.connected && !githubRepositories.value.length) loadGithubRepositories();
  const repository = githubRepositories.value.find((item) => item.full_name === repositoryForm.value.github_full_name);
  if (repository) loadGithubBranches(repository);
  loadDeployKey();
}

async function loadDeployKey() {
  if (!selectedSite.value) return;
  try {
    const response = await fetch(`/releasestation/api/v1/git/deploy-key/${selectedSite.value.id}`, { headers: { Accept: 'application/json' } });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) return;
    deployPublicKey.value = payload.data?.public_key || '';
    deployKeyFingerprint.value = payload.data?.fingerprint || '';
  } catch {
    // The repository editor remains usable when the optional SSH key is absent.
  }
}

async function generateDeployKey() {
  deployKeyLoading.value = true;
  repositoryError.value = '';
  try {
    const response = await fetch('/releasestation/api/v1/git/generate-deploy-key', { method: 'POST', headers: { Accept: 'application/json' } });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Nu am putut genera cheia deploy.');
    deployPublicKey.value = payload.data?.public_key || '';
    deployKeyFingerprint.value = payload.data?.fingerprint || '';
    repositoryMessage.value = 'Cheia publică a fost generată. Adaug-o în GitHub Deploy keys înainte de test.';
  } catch (error) {
    repositoryError.value = error instanceof Error ? error.message : 'Nu am putut genera cheia deploy.';
  } finally {
    deployKeyLoading.value = false;
  }
}

async function testGitTransport() {
  if (!selectedSite.value) return;
  gitTransportState.value = 'testing';
  gitTransportMessage.value = '';
  try {
    const response = await fetch('/releasestation/api/v1/git/test', { method: 'POST', headers: { Accept: 'application/json', 'Content-Type': 'application/json' }, body: JSON.stringify({ site_id: selectedSite.value.id, clone_url: repositoryForm.value.clone_url, branch: repositoryForm.value.branch }) });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Conexiunea Git nu a putut fi verificată.');
    gitTransportState.value = 'success';
    gitTransportMessage.value = 'Repository-ul și branch-ul răspund corect.';
  } catch (error) {
    gitTransportState.value = 'error';
    gitTransportMessage.value = error instanceof Error ? error.message : 'Conexiunea Git nu a putut fi verificată.';
  }
}

const frameworkSettingOptions = [
  { value: 'laravel', label: 'Laravel' }, { value: 'symfony', label: 'Symfony' }, { value: 'wordpress', label: 'WordPress' },
  { value: 'php', label: 'PHP' }, { value: 'nextjs', label: 'Next.js' }, { value: 'html', label: 'HTML' }, { value: 'other', label: 'Other' }, { value: 'custom', label: 'Custom' },
];

function resetSiteSettingsForm(site: Site) {
  const known = frameworkSettingOptions.some((option) => option.value === site.framework);
  siteSettingsForm.value = {
    framework: known ? site.framework : (site.custom_framework ? 'custom' : 'other'),
    custom_framework: site.custom_framework || (!known ? frameworkLabel(site.framework) : ''),
    tags: [...(site.tags || [])],
    color: site.color || '#f28c3b',
    push_to_deploy: !!site.push_to_deploy,
    deploy_script: site.deploy_script || '',
    deployment_retention: site.deployment_retention ?? 4,
    health_check_url: site.health_check_url || '',
    shared_directories: [...(site.shared_directories || [])],
  };
  siteTagDraft.value = '';
  sharedDirectoryDraft.value = '';
}

function openSiteSettings() {
  if (!selectedSite.value) return;
  resetSiteSettingsForm(selectedSite.value);
  siteSettingsMessage.value = '';
  siteSettingsError.value = '';
  selectedSiteTab.value = 'Settings';
  loadSiteWebhook();
  nextTick(mountDeployScriptEditor);
}

async function loadSiteWebhook() {
  if (!selectedSite.value) return;
  siteWebhookLoading.value = true;
  siteWebhookError.value = '';
  try {
    const response = await fetch(`/releasestation/api/v1/sites/${selectedSite.value.id}/webhook`, { headers: { Accept: 'application/json' } });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Nu am putut încărca setarea webhook.');
    siteWebhook.value = payload.data || { site_id: selectedSite.value.id, provider: 'github', enabled: false, configured: false, secret_configured: false };
  } catch (error) {
    siteWebhookError.value = error instanceof Error ? error.message : 'Nu am putut încărca setarea webhook.';
  } finally {
    siteWebhookLoading.value = false;
  }
}

async function rotateSiteWebhook() {
  if (!selectedSite.value) return;
  const provider = selectedSite.value.repository?.provider === 'gitlab' ? 'gitlab' : 'github';
  if (!window.confirm('Generezi credențiale noi? Secretul vechi va fi invalidat imediat.')) return;
  siteWebhookRotating.value = true;
  siteWebhookMessage.value = '';
  siteWebhookError.value = '';
  siteWebhookSecret.value = '';
  try {
    const response = await fetch(`/releasestation/api/v1/sites/${selectedSite.value.id}/webhook`, { method: 'POST', headers: { Accept: 'application/json', 'Content-Type': 'application/json' }, body: JSON.stringify({ provider }) });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Nu am putut genera webhook-ul.');
    siteWebhook.value = payload.data?.webhook || siteWebhook.value;
    siteWebhookSecret.value = payload.data?.secret || '';
    siteWebhookMessage.value = 'Webhook-ul este activ. Copiază secretul acum și configurează-l în provider.';
  } catch (error) {
    siteWebhookError.value = error instanceof Error ? error.message : 'Nu am putut genera webhook-ul.';
  } finally {
    siteWebhookRotating.value = false;
  }
}

async function copyWebhookValue(value: string) {
  if (!value) return;
  await navigator.clipboard?.writeText(value);
  siteWebhookMessage.value = 'Valoarea a fost copiată în clipboard.';
}

const deployScriptLineCount = computed(() => Math.max(1, siteSettingsForm.value.deploy_script.split('\n').length));

function destroyDeployScriptEditor() {
  deployScriptEditorView?.destroy();
  deployScriptEditorView = null;
}

function syncDeployScriptEditor() {
  if (!deployScriptEditorView) return;
  const value = siteSettingsForm.value.deploy_script;
  if (deployScriptEditorView.state.doc.toString() === value) return;
  deployScriptEditorView.dispatch({ changes: { from: 0, to: deployScriptEditorView.state.doc.length, insert: value } });
}

function mountDeployScriptEditor() {
  if (!deployScriptEditor.value) return;
  destroyDeployScriptEditor();
  deployScriptEditorView = new EditorView({
    state: EditorState.create({
      doc: siteSettingsForm.value.deploy_script,
      extensions: [
        basicSetup,
        oneDark,
        StreamLanguage.define(shell),
        EditorView.lineWrapping,
        EditorState.tabSize.of(2),
        EditorView.theme({
          '&': { minHeight: '310px', backgroundColor: '#0b111b', color: '#d9e7f5' },
          '.cm-scroller': { overflow: 'auto', fontFamily: "'DM Mono', monospace", fontSize: '11px', lineHeight: '1.7' },
          '.cm-content': { minHeight: '310px', padding: '16px 0' },
          '.cm-gutters': { minHeight: '310px', border: '0', borderRight: '1px solid #1d2a3b', backgroundColor: '#0b111b', color: '#60748c' },
          '.cm-activeLineGutter': { color: '#f2a05d', backgroundColor: '#111b2a' },
          '.cm-activeLine': { backgroundColor: 'rgba(84, 129, 177, .08)' },
          '.cm-selectionBackground, ::selection': { backgroundColor: 'rgba(242, 140, 59, .28) !important' },
          '.cm-focused': { outline: 'none' },
        }),
        EditorView.updateListener.of((update) => {
          if (update.docChanged) siteSettingsForm.value.deploy_script = update.state.doc.toString();
        }),
      ],
    }),
    parent: deployScriptEditor.value,
  });
}

function addSiteTag() {
  const tag = siteTagDraft.value.trim().replace(/\s+/g, ' ');
  if (!tag || [...tag].length > 50) return;
  if (!siteSettingsForm.value.tags.some((item) => item.toLowerCase() === tag.toLowerCase())) siteSettingsForm.value.tags.push(tag);
  siteTagDraft.value = '';
}

function onSiteTagKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === ',') { event.preventDefault(); addSiteTag(); }
}

function removeSiteTag(tag: string) {
  siteSettingsForm.value.tags = siteSettingsForm.value.tags.filter((item) => item !== tag);
}

function addSharedDirectory() {
  const value = sharedDirectoryDraft.value.trim().replace(/\\/g, '/').replace(/^\/+|\/+$/g, '');
  if (!value || value.includes(',') || value === '.' || value === '..' || value.startsWith('../') || value.startsWith('.zion') || value.startsWith('.current')) return;
  if (!siteSettingsForm.value.shared_directories.some((item) => item === value)) siteSettingsForm.value.shared_directories.push(value);
  sharedDirectoryDraft.value = '';
}

function onSharedDirectoryKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === ',') { event.preventDefault(); addSharedDirectory(); }
}

function removeSharedDirectory(directory: string) {
  siteSettingsForm.value.shared_directories = siteSettingsForm.value.shared_directories.filter((item) => item !== directory);
}

async function copySiteDirectory(value: string) {
  if (value) await navigator.clipboard?.writeText(value);
}

async function saveSiteSettings() {
  if (!selectedSite.value) return;
  if (siteSettingsForm.value.framework === 'custom' && !siteSettingsForm.value.custom_framework.trim()) { siteSettingsError.value = 'Introdu un nume pentru framework-ul custom.'; return; }
  if (siteSettingsForm.value.custom_framework.length > 100) { siteSettingsError.value = 'Framework-ul custom poate avea maximum 100 de caractere.'; return; }
  siteSettingsSaving.value = true; siteSettingsMessage.value = ''; siteSettingsError.value = '';
  try {
    const response = await fetch(`/releasestation/api/v1/sites/${selectedSite.value.id}/settings`, { method: 'PUT', headers: { Accept: 'application/json', 'Content-Type': 'application/json' }, body: JSON.stringify({ ...siteSettingsForm.value }) });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Nu am putut salva setările site-ului.');
    await loadSites();
    selectedSite.value = sites.value.find((site) => site.id === selectedSite.value?.id) || selectedSite.value;
    resetSiteSettingsForm(selectedSite.value);
    syncDeployScriptEditor();
    siteSettingsMessage.value = 'Setările site-ului au fost salvate.';
  } catch (error) { siteSettingsError.value = error instanceof Error ? error.message : 'Nu am putut salva setările site-ului.'; }
  finally { siteSettingsSaving.value = false; }
}

function selectSiteGithubRepository(fullName: string) {
  const repository = githubRepositories.value.find((item) => item.full_name === fullName);
  if (!repository) {
    githubBranches.value = [];
    githubBranchesError.value = '';
    return;
  }
  repositoryForm.value = {
    ...repositoryForm.value,
    provider: 'github',
    clone_url: repository.clone_url,
    branch: repository.default_branch,
    github_installation_id: repository.installation_id,
    github_repository_id: repository.id,
    github_full_name: repository.full_name,
    github_default_branch: repository.default_branch,
  };
  loadGithubBranches(repository);
}

async function saveSiteRepository() {
  if (!selectedSite.value) return;
  if (!repositoryForm.value.clone_url.trim() || !repositoryForm.value.branch.trim()) {
    repositoryError.value = 'Completează URL-ul repository-ului și branch-ul.';
    return;
  }
  repositorySaving.value = true;
  repositoryMessage.value = '';
  repositoryError.value = '';
  try {
    const response = await fetch(`/releasestation/api/v1/sites/${selectedSite.value.id}/repository`, {
      method: 'PUT',
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: JSON.stringify({ strategy: repositoryForm.value.strategy, repository: { provider: repositoryForm.value.provider, clone_url: repositoryForm.value.clone_url, branch: repositoryForm.value.branch, github_installation_id: repositoryForm.value.github_installation_id, github_repository_id: repositoryForm.value.github_repository_id, github_full_name: repositoryForm.value.github_full_name, github_default_branch: repositoryForm.value.github_default_branch } }),
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Nu am putut salva repository-ul.');
    await loadSites();
    selectedSite.value = sites.value.find((site) => site.id === selectedSite.value?.id) || selectedSite.value;
    resetRepositoryForm(selectedSite.value);
    await loadSiteHistory();
    repositoryMessage.value = 'Repository-ul și metoda de deployment au fost salvate.';
  } catch (error) {
    repositoryError.value = error instanceof Error ? error.message : 'Nu am putut salva repository-ul.';
  } finally {
    repositorySaving.value = false;
  }
}

async function disconnectSiteRepository() {
  if (!selectedSite.value || !window.confirm('Elimini repository-ul asociat acestui site?')) return;
  repositorySaving.value = true;
  repositoryMessage.value = '';
  repositoryError.value = '';
  try {
    const response = await fetch(`/releasestation/api/v1/sites/${selectedSite.value.id}/repository`, { method: 'DELETE', headers: { Accept: 'application/json' } });
    if (!response.ok) throw new Error('Nu am putut elimina repository-ul.');
    await loadSites();
    selectedSite.value = sites.value.find((site) => site.id === selectedSite.value?.id) || selectedSite.value;
    resetRepositoryForm(selectedSite.value);
    repositoryMessage.value = 'Repository-ul a fost eliminat. Site-ul rămâne în catalog.';
  } catch (error) {
    repositoryError.value = error instanceof Error ? error.message : 'Nu am putut elimina repository-ul.';
  } finally {
    repositorySaving.value = false;
  }
}

async function loadSiteHistory() {
  if (!selectedSite.value) return;
  siteDetailLoading.value = true;
  releasesLoading.value = true;
  try {
    const [deploymentResponse, commitResponse, releaseResponse] = await Promise.all([
      fetch(`/releasestation/api/v1/sites/${selectedSite.value.id}/deployments?page=${sitePage.value}&per_page=25&q=${encodeURIComponent(deploymentSearch.value)}`, { headers: { Accept: 'application/json' } }),
      fetch(`/releasestation/api/v1/sites/${selectedSite.value.id}/commits`, { headers: { Accept: 'application/json' } }),
      fetch(`/releasestation/api/v1/sites/${selectedSite.value.id}/releases`, { headers: { Accept: 'application/json' } }),
    ]);
    const deploymentPayload = await deploymentResponse.json().catch(() => ({}));
    const commitPayload = await commitResponse.json().catch(() => ({}));
    const releasePayload = await releaseResponse.json().catch(() => ({}));
    if (!deploymentResponse.ok) throw new Error(deploymentPayload.error?.message || 'Could not load deployments.');
    if (!commitResponse.ok) throw new Error(commitPayload.error?.message || 'Could not load repository commits.');
    if (!releaseResponse.ok) throw new Error(releasePayload.error?.message || 'Could not load releases.');
    siteDeployments.value = deploymentPayload.data?.items || [];
    siteTotalPages.value = deploymentPayload.data?.total_pages || 1;
    siteCommits.value = commitResponse.ok ? (commitPayload.data || []) : [];
    siteReleases.value = releaseResponse.ok ? (releasePayload.data || []) : [];
  } catch (error) {
    deployError.value = error instanceof Error ? error.message : 'Could not load site history.';
  } finally {
    siteDetailLoading.value = false;
    releasesLoading.value = false;
  }
}

async function rollbackRelease(release: Release) {
  if (!selectedSite.value || release.active || rollbackReleaseID.value) return;
  const commit = release.commit_sha ? release.commit_sha.slice(0, 7) : release.id;
  if (!window.confirm(`Rollback application files to ${commit}? Database migrations will not be reversed.`)) return;
  rollbackReleaseID.value = release.id;
  releaseMessage.value = '';
  releaseError.value = '';
  try {
    const response = await fetch(`/releasestation/api/v1/sites/${selectedSite.value.id}/rollback`, {
      method: 'POST', headers: { Accept: 'application/json', 'Content-Type': 'application/json' }, body: JSON.stringify({ release_id: release.id }),
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Rollback failed.');
    releaseMessage.value = `Application files rolled back to ${commit}. Database migrations were not reversed.`;
    await loadSites();
    selectedSite.value = sites.value.find((site) => site.id === selectedSite.value?.id) || selectedSite.value;
    await loadSiteHistory();
  } catch (error) {
    releaseError.value = error instanceof Error ? error.message : 'Rollback failed.';
  } finally {
    rollbackReleaseID.value = '';
  }
}

function openSite(site: Site) {
  selectedSite.value = site;
  selectedSiteTab.value = 'Overview';
  selectedDeployment.value = null;
  sitePage.value = 1;
  deploymentSearch.value = '';
  activeNav.value = 'SiteDetail';
  loadSiteHistory();
}

function closeSiteDetail() {
  selectedSite.value = null;
  selectedDeployment.value = null;
  activeNav.value = 'Sites';
}

async function openDeploymentDetails(deployment: Deployment) {
  if (!selectedSite.value) return;
  selectedDeployment.value = deployment;
  logsExpanded.value = true;
  activeNav.value = 'DeploymentDetail';
  deploymentDetailLoading.value = true;
  try {
    const response = await fetch(`/releasestation/api/v1/sites/${selectedSite.value.id}/deployments/${deployment.id}`, { headers: { Accept: 'application/json' } });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Could not load deployment details.');
    selectedDeployment.value = payload.data;
    if (!deploymentTerminal(selectedDeployment.value?.status)) watchDeploymentStream(deployment.id);
  } catch (error) {
    deployError.value = error instanceof Error ? error.message : 'Could not load deployment details.';
  } finally {
    deploymentDetailLoading.value = false;
  }
}

function deploymentTerminal(status?: string) {
  return status === 'deployed' || status === 'failed' || status === 'rolled_back' || status === 'cancelled';
}

function stopDeploymentStream() {
  deploymentEventSource?.close();
  deploymentEventSource = undefined;
}

function setDeploymentNotice(id: string, siteID: string, siteName: string, status: string, message?: string) {
  const progress = status === 'queued' ? 18 : status === 'running' ? 52 : 100;
  deploymentNotice.value = { id, siteID, siteName, status, progress, message: message || `Deployment ${status}. Click to follow live logs.` };
  if (deploymentNoticeTimer) clearTimeout(deploymentNoticeTimer);
  if (deploymentTerminal(status)) {
    deploymentNoticeTimer = setTimeout(() => { deploymentNotice.value = null; }, 9000);
  }
}

function openDeploymentNotice() {
  const notice = deploymentNotice.value;
  if (!notice) return;
  const site = sites.value.find((item) => item.id === notice.siteID);
  if (!site) {
    activeNav.value = 'Sites';
    return;
  }
  selectedSite.value = site;
  selectedSiteTab.value = 'Deployments';
  activeNav.value = 'DeploymentDetail';
  openDeploymentDetails({ id: notice.id } as Deployment);
}

function dismissDeploymentNotice() {
  if (deploymentNoticeTimer) clearTimeout(deploymentNoticeTimer);
  deploymentNotice.value = null;
}

function watchDeploymentStream(deploymentID: string, siteID = selectedSite.value?.id) {
  stopDeploymentStream();
  if (!siteID) return;
  const source = new EventSource(`/releasestation/api/v1/sites/${siteID}/deployments/${deploymentID}/logs/stream`);
  deploymentEventSource = source;
  source.addEventListener('snapshot', (event) => {
    try {
      const detail = JSON.parse((event as MessageEvent).data) as Deployment;
      if (selectedDeployment.value?.id === deploymentID) selectedDeployment.value = detail;
      setDeploymentNotice(deploymentID, siteID, sites.value.find((item) => item.id === siteID)?.name || siteID, detail.status, detail.status === 'deployed' ? 'Deployment completed successfully.' : undefined);
      if (deploymentTerminal(detail.status)) {
        source.close();
        loadSiteHistory();
      }
    } catch { /* Keep the stream alive when a frame is malformed. */ }
  });
  source.addEventListener('status', (event) => {
    try {
      const update = JSON.parse((event as MessageEvent).data) as { status: string };
      if (selectedDeployment.value?.id === deploymentID && selectedDeployment.value) selectedDeployment.value = { ...selectedDeployment.value, status: update.status };
      setDeploymentNotice(deploymentID, siteID, sites.value.find((item) => item.id === siteID)?.name || siteID, update.status, update.status === 'failed' ? 'Deployment failed. Click to inspect the error logs.' : undefined);
      loadSiteHistory();
      if (deploymentTerminal(update.status)) source.close();
    } catch { /* Keep the stream alive when a frame is malformed. */ }
  });
  source.addEventListener('log', (event) => {
    try {
      const update = JSON.parse((event as MessageEvent).data) as { channel?: string; message?: string };
      if (!selectedDeployment.value || selectedDeployment.value.id !== deploymentID || !update.message) return;
      const key = update.channel === 'build' ? 'build_log' : 'deployment_log';
      selectedDeployment.value = { ...selectedDeployment.value, [key]: `${selectedDeployment.value[key] || ''}${update.message}\n` };
    } catch { /* Keep the stream alive when a frame is malformed. */ }
  });
  source.onerror = () => {
    if (deploymentEventSource === source && selectedDeployment.value && deploymentTerminal(selectedDeployment.value.status)) stopDeploymentStream();
  };
}

function backToSiteTab(tab: 'Overview' | 'Deployments' = 'Overview') {
  selectedDeployment.value = null;
  selectedSiteTab.value = tab;
  activeNav.value = 'SiteDetail';
}

async function deployCommit(commit: Commit) {
    if (!selectedSite.value || deployingCommitSha.value) return;
  deployingCommitSha.value = commit.sha;
  deployMessage.value = '';
  deployError.value = '';
  try {
    const response = await fetch(`/releasestation/api/v1/sites/${selectedSite.value.id}/deploy`, {
      method: 'POST', headers: { Accept: 'application/json', 'Content-Type': 'application/json' }, body: JSON.stringify({ ref: commit.sha }),
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Deployment failed.');
    deployMessage.value = `${commit.sha.slice(0, 7)} a fost pus în coadă pentru deployment.`;
    if (payload.data?.id) setDeploymentNotice(payload.data.id, selectedSite.value.id, selectedSite.value.hostname || selectedSite.value.name, payload.data.status || 'queued');
    scheduleDeploymentToastDismissal();
    if (payload.data?.id) watchDeploymentStream(payload.data.id, selectedSite.value.id);
    await loadSiteHistory();
  } catch (error) {
    deployError.value = error instanceof Error ? error.message : 'Deployment failed.';
    scheduleDeploymentToastDismissal();
    await loadSiteHistory();
  } finally {
    deployingCommitSha.value = '';
  }
}

function setDeploymentPage(page: number) {
  if (page < 1 || page > siteTotalPages.value) return;
  sitePage.value = page;
  loadSiteHistory();
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

async function loadGithubStatus() {
  if (!githubPairingHandled) await completeGithubPairingFromUrl();
  try {
    const response = await fetch('/releasestation/api/v1/integrations/github', { headers: { Accept: 'application/json' } });
    if (!response.ok) throw new Error('github');
    const payload = await response.json();
    githubState.value = payload.data || emptyGithubState();
    if (githubState.value.connected && !githubRepositories.value.length) loadGithubRepositories();
  } catch {
    githubState.value = emptyGithubState();
  }
}

async function completeGithubPairingFromUrl() {
  const url = new URL(window.location.href);
  const pairingCode = url.searchParams.get('pairing_code');
  if (!pairingCode || githubPairingHandled) return;
  githubPairingHandled = true;
  try {
    const response = await fetch('/releasestation/api/v1/integrations/github/complete', {
      method: 'POST',
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: JSON.stringify({ pairing_code: pairingCode }),
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Nu am putut finaliza conectarea Zion.');
    url.searchParams.delete('pairing_code');
    window.history.replaceState({}, document.title, url.toString());
    githubMessage.value = 'GitHub a fost conectat prin Zion. Repository-urile permise sunt disponibile.';
  } catch (error) {
    githubError.value = error instanceof Error ? error.message : 'Nu am putut finaliza conectarea Zion.';
  }
}

async function loadGithubRepositories() {
  githubLoading.value = true;
  githubError.value = '';
  try {
    const response = await fetch('/releasestation/api/v1/integrations/github/repositories', { headers: { Accept: 'application/json' } });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Nu am putut citi repository-urile acordate GitHub App.');
    githubRepositories.value = payload.data || [];
    if (selectedSiteTab.value === 'Repository') {
      const repository = githubRepositories.value.find((item) => item.full_name === repositoryForm.value.github_full_name);
      if (repository) loadGithubBranches(repository);
    }
  } catch (error) {
    githubError.value = error instanceof Error ? error.message : 'Nu am putut citi repository-urile GitHub.';
  } finally {
    githubLoading.value = false;
  }
}

function selectGithubRepository(fullName: string) {
  const repository = githubRepositories.value.find((item) => item.full_name === fullName);
  if (!repository) {
    wizardForm.value.githubInstallationId = null;
    wizardForm.value.githubRepositoryId = null;
    wizardForm.value.githubFullName = '';
    wizardForm.value.githubDefaultBranch = '';
    githubBranches.value = [];
    githubBranchesError.value = '';
    return;
  }
  wizardForm.value.provider = 'github';
  wizardForm.value.githubInstallationId = repository.installation_id;
  wizardForm.value.githubRepositoryId = repository.id;
  wizardForm.value.githubFullName = repository.full_name;
  wizardForm.value.githubDefaultBranch = repository.default_branch;
  wizardForm.value.cloneUrl = repository.clone_url;
  wizardForm.value.branch = repository.default_branch;
  loadGithubBranches(repository);
}

async function loadGithubBranches(repository: GithubRepository) {
  githubBranchesLoading.value = true;
  githubBranchesError.value = '';
  try {
    const [owner, name] = repository.full_name.split('/');
    const siteQuery = selectedSite.value?.id ? `&site_id=${encodeURIComponent(selectedSite.value.id)}` : '';
    const response = await fetch(`/releasestation/api/v1/integrations/github/repositories/${encodeURIComponent(owner)}/${encodeURIComponent(name)}/branches?installation_id=${repository.installation_id}${siteQuery}`, { headers: { Accept: 'application/json' } });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Nu am putut citi branch-urile repository-ului.');
    githubBranches.value = Array.isArray(payload.data) ? payload.data : [];
    if (!githubBranches.value.length) throw new Error('Repository-ul nu are branch-uri disponibile.');
    if (!githubBranches.value.includes(repository.default_branch)) githubBranches.value.unshift(repository.default_branch);
    if (!githubBranches.value.includes(wizardForm.value.branch)) wizardForm.value.branch = repository.default_branch;
    if (repositoryForm.value.github_full_name === repository.full_name && !githubBranches.value.includes(repositoryForm.value.branch)) repositoryForm.value.branch = repository.default_branch;
  } catch (error) {
    githubBranches.value = [repository.default_branch];
    githubBranchesError.value = error instanceof Error ? error.message : 'Nu am putut citi branch-urile repository-ului.';
  } finally {
    githubBranchesLoading.value = false;
  }
}

function onGithubRepositoryChange(event: Event) {
  selectGithubRepository((event.target as HTMLSelectElement).value);
}

async function installGithubApp() {
  githubError.value = '';
  githubMessage.value = '';
  githubConnectState.value = 'starting';
  const authWindow = window.open('about:blank', 'zion-github-authorization');
  try {
    const response = await fetch('/releasestation/api/v1/integrations/github/install', { method: 'POST', headers: { Accept: 'application/json' } });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Nu am putut porni instalarea GitHub App.');
    if (authWindow) authWindow.location.href = payload.data.url;
    else window.location.href = payload.data.url;
    if (payload.data.mode === 'pairing' && payload.data.session_id && payload.data.poll_token) {
      githubPairingSessionId = payload.data.session_id;
      githubPairingToken = payload.data.poll_token;
      githubConnectState.value = 'waiting';
      startGithubPairingPolling();
    } else if (payload.data.mode === 'managed') {
      githubConnectState.value = 'waiting';
      startGithubStatusPolling();
    } else {
      githubConnectState.value = 'idle';
    }
  } catch (error) {
    githubConnectState.value = 'idle';
    githubError.value = error instanceof Error ? error.message : 'Nu am putut porni instalarea GitHub App.';
  }
}

function startGithubPairingPolling() {
  if (githubPollTimer) clearInterval(githubPollTimer);
  let attempts = 0;
  githubPollTimer = setInterval(async () => {
    attempts += 1;
    try {
      const response = await fetch('/releasestation/api/v1/integrations/github/pairing-status', {
        method: 'POST',
        headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_id: githubPairingSessionId, poll_token: githubPairingToken }),
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(payload.error?.message || 'Nu am putut verifica pairing-ul GitHub.');
      if (payload.data?.connected) {
        clearInterval(githubPollTimer);
        githubPollTimer = undefined;
        githubConnectState.value = 'idle';
        githubMessage.value = 'GitHub a fost conectat. Repository-urile private permise sunt disponibile.';
        await loadGithubStatus();
        loadGithubRepositories();
      } else if (attempts >= 120) {
        clearInterval(githubPollTimer);
        githubPollTimer = undefined;
        githubConnectState.value = 'idle';
        githubError.value = 'Pairing-ul GitHub a expirat. Încearcă din nou.';
      }
    } catch (error) {
      clearInterval(githubPollTimer);
      githubPollTimer = undefined;
      githubConnectState.value = 'idle';
      githubError.value = error instanceof Error ? error.message : 'Nu am putut verifica pairing-ul GitHub.';
    }
  }, 3000);
}

function startGithubStatusPolling() {
  if (githubPollTimer) clearInterval(githubPollTimer);
  let attempts = 0;
  githubPollTimer = setInterval(async () => {
    attempts += 1;
    await loadGithubStatus();
    if (githubState.value.connected || attempts >= 120) {
      if (githubPollTimer) clearInterval(githubPollTimer);
      githubPollTimer = undefined;
      githubConnectState.value = 'idle';
      if (githubState.value.connected) {
        githubMessage.value = 'GitHub a fost conectat. Repository-urile private permise sunt disponibile.';
        loadGithubRepositories();
      }
    }
  }, 5000);
}

function openWizard() {
  discoveryOpen.value = false;
  wizardOpen.value = true;
  wizardStep.value = 1;
  wizardError.value = '';
  if (githubState.value.connected && !githubRepositories.value.length) loadGithubRepositories();
}

function closeWizard() {
  if (!wizardSaving.value) wizardOpen.value = false;
}

function nextWizardStep() {
  wizardError.value = '';
  if (wizardStep.value === 1) {
    if (!wizardForm.value.name.trim() || !wizardForm.value.url.trim() || !wizardForm.value.projectRoot.trim()) {
      wizardError.value = 'Completează numele, URL-ul public și calea proiectului de pe NAS.';
      return;
    }
    try {
      const parsed = new URL(wizardForm.value.url);
      if (!['http:', 'https:'].includes(parsed.protocol) || !parsed.hostname) throw new Error('url');
    } catch {
      wizardError.value = 'URL-ul trebuie să înceapă cu http:// sau https:// și să conțină un domeniu valid.';
      return;
    }
  }
  if (wizardStep.value === 2 && (!wizardForm.value.cloneUrl.trim() || !wizardForm.value.branch.trim())) {
    wizardError.value = 'Adaugă URL-ul repository-ului și branch-ul folosit la deploy.';
    return;
  }
  if (wizardStep.value < 4) wizardStep.value += 1;
}

function previousWizardStep() {
  wizardError.value = '';
  if (wizardStep.value > 1) wizardStep.value -= 1;
}

async function createManualSite() {
  wizardSaving.value = true;
  wizardError.value = '';
  try {
    const response = await fetch('/releasestation/api/v1/sites', {
      method: 'POST',
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: wizardForm.value.name,
        url: wizardForm.value.url,
        project_root: wizardForm.value.projectRoot,
        web_root: wizardForm.value.webRoot || undefined,
        framework: wizardForm.value.framework,
        strategy: wizardForm.value.strategy,
        repository: {
          provider: wizardForm.value.provider,
          clone_url: wizardForm.value.cloneUrl,
          branch: wizardForm.value.branch,
          github_installation_id: wizardForm.value.githubInstallationId,
          github_repository_id: wizardForm.value.githubRepositoryId,
          github_full_name: wizardForm.value.githubFullName,
          github_default_branch: wizardForm.value.githubDefaultBranch,
        },
      }),
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Nu am putut salva site-ul.');
    await loadSites();
    wizardOpen.value = false;
    const created = sites.value.find((site) => site.id === payload.data?.id);
    if (created) {
      openSite(created);
    } else {
      activeNav.value = 'Sites';
    }
  } catch (error) {
    wizardError.value = error instanceof Error ? error.message : 'Nu am putut salva site-ul.';
  } finally {
    wizardSaving.value = false;
  }
}

async function saveGithubConnection() {
  githubSaving.value = true;
  githubMessage.value = '';
  githubError.value = '';
  try {
    const response = await fetch('/releasestation/api/v1/integrations/github', {
      method: 'PUT',
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: JSON.stringify({ account: githubAccount.value }),
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Nu am putut salva conexiunea GitHub.');
    githubState.value = payload.data;
    githubMessage.value = 'Conexiunea GitHub a fost salvată.';
  } catch (error) {
    githubError.value = error instanceof Error ? error.message : 'Nu am putut salva conexiunea GitHub.';
  } finally {
    githubSaving.value = false;
  }
}

async function disconnectGithub() {
  githubSaving.value = true;
  githubMessage.value = '';
  githubError.value = '';
  try {
    const response = await fetch('/releasestation/api/v1/integrations/github', { method: 'DELETE' });
    if (!response.ok) throw new Error('Nu am putut deconecta GitHub.');
    githubState.value = emptyGithubState();
    githubAccount.value = '';
    githubMessage.value = 'Conexiunea GitHub a fost eliminată.';
  } catch (error) {
    githubError.value = error instanceof Error ? error.message : 'Nu am putut deconecta GitHub.';
  } finally {
    githubSaving.value = false;
  }
}

async function archiveSite(site: Site) {
  if (!window.confirm(`Archive ${site.hostname || site.name}?`)) return;
  const response = await fetch(`/releasestation/api/v1/sites/${site.id}`, { method: 'DELETE' });
  if (response.ok) await loadSites();
}

async function deploySite(site: Site) {
  deployMessage.value = '';
  deployError.value = '';
  deployingSiteId.value = site.id;
  try {
    const response = await fetch(`/releasestation/api/v1/sites/${site.id}/deploy`, { method: 'POST', headers: { Accept: 'application/json' } });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || 'Deployment failed.');
    deployMessage.value = `${site.hostname || site.name} a fost pus în coadă pentru deployment.`;
    if (payload.data?.id) setDeploymentNotice(payload.data.id, site.id, site.hostname || site.name, payload.data.status || 'queued');
    scheduleDeploymentToastDismissal();
    if (payload.data?.id) watchDeploymentStream(payload.data.id, site.id);
    await loadSites();
  } catch (error) {
    deployError.value = error instanceof Error ? error.message : 'Deployment failed.';
    scheduleDeploymentToastDismissal();
  } finally {
    deployingSiteId.value = '';
  }
}

function dismissDeploymentToast() {
  deployMessage.value = '';
  deployError.value = '';
}

function scheduleDeploymentToastDismissal() {
  if (deploymentToastTimer) clearTimeout(deploymentToastTimer);
  deploymentToastTimer = setTimeout(() => {
    deploymentToastTimer = undefined;
    dismissDeploymentToast();
  }, 5000);
}

function clearImportCloseTimer() {
  if (importCloseTimer) {
    clearTimeout(importCloseTimer);
    importCloseTimer = undefined;
  }
}

function closeDiscovery() {
  clearImportCloseTimer();
  importClosing.value = false;
  importCountdown.value = 5;
  discoveryOpen.value = false;
}

function selectAllDiscovered() {
  selectedDiscoveredPaths.value = [...selectableDiscoveredPaths.value];
}

function selectNoneDiscovered() {
  selectedDiscoveredPaths.value = [];
}

function startImportCloseCountdown() {
  clearImportCloseTimer();
  importClosing.value = true;
  importCountdown.value = 5;
  const tick = () => {
    importCountdown.value -= 1;
    if (importCountdown.value <= 0) {
      importCloseTimer = window.setTimeout(closeDiscovery, 450);
      return;
    }
    importCloseTimer = window.setTimeout(tick, 1000);
  };
  importCloseTimer = window.setTimeout(tick, 1000);
}

async function openDiscovery() {
  clearImportCloseTimer();
  importClosing.value = false;
  importCountdown.value = 5;
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
    const payload = await response.json().catch(() => null);
    if (!response.ok) throw new Error(payload?.error?.message || 'Discovery endpoint returned an error.');
    if (!payload || !Array.isArray(payload.data)) throw new Error('The installed SPK does not expose the Web Station discovery API. Install the latest package first.');
    discoveredSites.value = payload.data || [];
    selectedDiscoveredPaths.value = discoveredSites.value.filter((site) => !site.already_managed).map((site) => site.project_root);
    discoveryPhase.value = discoveredSites.value.length ? 'Review discovered applications' : 'No applications found';
  } catch (error) {
    discoveryError.value = error instanceof Error ? error.message : 'Web Station discovery is unavailable. Verify the configured read-only roots.';
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
  discoveryPhase.value = `Importing ${selectedDiscoveredPaths.value.length} selected site${selectedDiscoveredPaths.value.length === 1 ? '' : 's'}`;
  console.info('[ReleaseStation] Web Station import started', { paths: selectedDiscoveredPaths.value });
  try {
    const response = await fetch('/releasestation/api/v1/webstation/import', {
      method: 'POST',
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: JSON.stringify({ paths: selectedDiscoveredPaths.value }),
    });
    const payload = await response.json().catch(() => null);
    console.info('[ReleaseStation] Web Station import response', { status: response.status, ok: response.ok, payload });
    if (!response.ok) throw new Error(payload?.error?.message || `Import endpoint returned HTTP ${response.status}.`);
    const imported = payload?.data?.imported || [];
    const skipped = payload?.data?.skipped || [];
    importMessage.value = `${imported.length} site${imported.length === 1 ? '' : 's'} imported${skipped.length ? `, ${skipped.length} already managed` : ''}.`;
    await loadSites();
    selectedDiscoveredPaths.value = [];
    discoveredSites.value = discoveredSites.value.map((site) => ({ ...site, already_managed: imported.some((item: Site) => item.project_root === site.project_root) || site.already_managed }));
    window.requestAnimationFrame(() => discoveryDialog.value?.scrollTo({ top: 0, behavior: 'smooth' }));
    startImportCloseCountdown();
  } catch (error) {
    clearImportCloseTimer();
    importClosing.value = false;
    discoveryPhase.value = 'Import failed';
    discoveryError.value = error instanceof Error ? `Import failed: ${error.message}` : 'Import failed. Check the site permissions and try again.';
    console.error('[ReleaseStation] Web Station import failed', error);
  } finally {
    discoveryLoading.value = false;
  }
}

onMounted(() => {
  applyTheme();
  document.addEventListener('keydown', onKeydown);
  checkHealth();
  loadSites();
  loadDashboardMetrics();
  loadAuditLogs();
  loadSystemChecks();
  loadWebStationStatus();
  loadGithubStatus();
  dashboardRefreshTimer = setInterval(() => {
    checkHealth();
    loadSites();
    loadDashboardMetrics();
    loadAuditLogs();
    loadWebStationStatus();
  }, 5000);
});

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown);
  if (githubPollTimer) clearInterval(githubPollTimer);
  if (dashboardRefreshTimer) clearInterval(dashboardRefreshTimer);
  clearImportCloseTimer();
  if (deploymentToastTimer) clearTimeout(deploymentToastTimer);
  if (deploymentNoticeTimer) clearTimeout(deploymentNoticeTimer);
  stopDeploymentStream();
  destroyDeployScriptEditor();
});
</script>

<template>
  <div class="app-frame">
    <aside class="sidebar">
      <div class="brand-lockup">
        <img class="brand-icon" :src="packageIconURL" alt="Zion Release Station" />
        <div>
          <div class="brand-name">Zion</div>
          <div class="brand-product">Release Station</div>
        </div>
      </div>

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
        <button class="nav-item" type="button" @click="activeNav = 'Sites'"><Webhook :size="17" /><span>Webhooks</span></button>
        <button class="nav-item" type="button"><ShieldCheck :size="17" /><span>Secrets</span><span class="nav-dot" /></button>
      </nav>

      <div class="sidebar-spacer" />
      <div class="sidebar-bottom">
        <button class="nav-item" :class="{ active: activeNav === 'Settings' }" type="button" @click="activeNav = 'Settings'"><Settings2 :size="17" /><span>Settings</span></button>
        <button class="nav-item" :class="{ active: activeNav === 'Help' }" type="button" @click="openHelp()"><LifeBuoy :size="17" /><span>Help center</span></button>
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
          <button class="icon-button theme-toggle" type="button" :aria-label="isDark ? 'Switch to light theme' : 'Switch to dark theme'" :title="isDark ? 'Light mode' : 'Dark mode'" @click="toggleTheme"><Sun v-if="isDark" :size="17" /><Moon v-else :size="17" /></button>
          <button class="icon-button" type="button" aria-label="Help" title="Help center" @click="openHelp()"><CircleHelp :size="17" /></button>
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
            <button class="button button-primary" type="button" @click="openWizard"><Plus :size="17" />New project</button>
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
            <div class="metric-value">{{ dashboardMetrics.successful_deploys }} <span class="metric-muted">/ {{ dashboardMetrics.total_deploys }}</span></div>
            <div class="metric-foot"><span class="status-inline"><span class="status-dot" />Real deployment data</span><span>All time</span></div>
          </article>
          <article class="metric-card">
            <div class="metric-top"><span class="metric-label">Median deploy time</span><span class="metric-icon violet"><Clock3 :size="16" /></span></div>
            <div class="metric-value">{{ dashboardMetrics.median_duration_ms ? (dashboardMetrics.median_duration_ms / 1000).toFixed(1) : '0.0' }}<span class="metric-unit">s</span></div>
            <div class="metric-foot"><span class="status-inline"><span class="status-dot" />Measured deployments</span><span>All time</span></div>
          </article>
          <article class="metric-card">
            <div class="metric-top"><span class="metric-label">Queue status</span><span class="metric-icon blue"><ListChecks :size="16" /></span></div>
            <div class="metric-value">{{ dashboardMetrics.queue_status === 'running' ? 'Running' : (dashboardMetrics.queue_status === 'queued' ? 'Queued' : 'Idle') }}</div>
            <div class="metric-foot"><span class="status-inline"><span class="status-dot" />{{ dashboardMetrics.running_deploys + dashboardMetrics.queued_deploys }} active</span><span>Live polling</span></div>
          </article>
        </section>

        <section v-if="activeNav === 'Dashboard'" class="dashboard-grid">
          <article class="panel pipeline-panel">
            <div class="panel-heading">
              <div><div class="panel-kicker"><span class="live-dot" /> LIVE DEPLOYMENT</div><h2>{{ featuredSite?.hostname || featuredSite?.name || 'No GitHub site configured' }}</h2></div>
              <button class="more-button" type="button"><MoreHorizontal :size="18" /></button>
            </div>
            <template v-if="dashboardMetrics.latest">
              <div class="deploy-meta"><span class="branch-chip"><GitBranch :size="14" />{{ dashboardMetrics.latest.branch || 'main' }}</span><code>{{ dashboardMetrics.latest.commit_sha ? dashboardMetrics.latest.commit_sha.slice(0, 7) : 'unknown' }}</code><span class="meta-separator">·</span><span>{{ relativeTime(dashboardMetrics.latest.created_at) }}</span><span class="deploy-state"><span class="status-dot" />{{ dashboardMetrics.latest.status }}</span></div>
              <div class="pipeline-track">
                <div class="pipeline-step">
                  <div class="step-marker"><Check :size="13" /></div>
                  <div class="step-copy"><strong>{{ dashboardMetrics.latest.commit_message || 'Deployment' }}</strong><span>{{ dashboardMetrics.latest.status }}</span></div>
                  <code>{{ durationLabel(dashboardMetrics.latest.duration_ms) }}</code>
                </div>
              </div>
              <div class="terminal-preview"><div class="terminal-header"><span><span class="terminal-dot red" /><span class="terminal-dot yellow" /><span class="terminal-dot green" /></span><span>latest deployment</span><span>{{ dashboardMetrics.latest.status }}</span></div><div class="terminal-body"><p><span class="terminal-muted">$</span> {{ dashboardMetrics.latest.commit_sha || 'no commit' }}</p><p class="terminal-success"><span>✓</span> {{ dashboardMetrics.latest.commit_message || 'Deployment recorded.' }}</p></div></div>
            </template>
            <div v-else class="management-empty dashboard-empty"><UploadCloud :size="22" /><strong>No deployments yet</strong><span>Configure a GitHub repository for a site, then deploy a commit to populate this live panel.</span>
            </div>
          </article>

          <article class="panel activity-panel">
            <div class="panel-heading"><div><div class="panel-kicker">SYSTEM OVERVIEW</div><h2>Everything is ready</h2></div><span class="health-badge" :class="healthState"><span class="status-dot" />{{ healthLabel }}</span></div>
            <div class="system-list">
              <div v-for="item in systemItems" :key="item.label" class="system-row"><span class="system-icon"><component :is="item.icon" :size="15" /></span><span class="system-copy"><strong>{{ item.label }}</strong><small>{{ item.detail }}</small></span><button class="system-read-more" type="button" @click="openHelp(item.label)">Read more <ArrowUpRight :size="12" /></button><Check v-if="item.state === 'ready'" :size="16" class="system-check" /><CircleX v-else :size="16" class="system-error" /></div>
            </div>
            <div class="system-foot"><span>DSM 7.4-90075</span><span class="system-foot-separator">·</span><span>x86_64 / apollolake</span><button type="button" @click="checkHealth"><RotateCw :size="14" />Refresh</button></div>
          </article>
        </section>

        <section v-if="activeNav === 'Dashboard' && githubState.mode === 'managed'" class="connection-grid">
          <article class="panel github-panel">
            <div class="panel-heading"><div><div class="panel-kicker"><GitBranch :size="13" /> SOURCE CONTROL</div><h2>GitHub connection</h2></div><span class="connection-badge" :class="{ connected: githubState.connected }"><span class="status-dot" />{{ githubState.connected ? 'Connected' : 'Not connected' }}</span></div>
            <p v-if="githubState.connected" class="connection-copy">GitHub este conectat prin Synology Connector și poate accesa repository-urile private permise în GitHub{{ githubState.account_login ? ` pentru ${githubState.account_login}` : '' }}. Webhook: {{ githubState.webhook_configured ? 'configured' : 'not configured' }}.</p>
            <p v-else class="connection-copy">Conectează GitHub prin Synology Connector. Vei fi trimis la GitHub să te autentifici și să instalezi aplicația „Synology Connector” în contul sau organizația ta.</p>
            <div class="connection-foot"><span class="muted-copy">{{ githubState.connected ? `${githubState.installations.length} installation${githubState.installations.length === 1 ? '' : 's'}` : (githubState.configuration_error || 'Ready to connect') }}</span><button class="button button-primary" type="button" @click="installGithubApp" :disabled="githubConnectState !== 'idle'">{{ githubConnectState === 'waiting' ? 'Waiting for GitHub…' : (githubState.connected ? 'Manage GitHub' : 'Connect GitHub') }} <ArrowUpRight :size="14" /></button></div>
          </article>
        </section>

        <section v-if="activeNav === 'Dashboard' && githubState.mode !== 'managed'" class="connection-grid">
          <article class="panel github-panel">
            <div class="panel-heading"><div><div class="panel-kicker"><GitBranch :size="13" /> SOURCE CONTROL</div><h2>GitHub connection</h2></div><span class="connection-badge" :class="{ connected: githubState.connected }"><span class="status-dot" />{{ githubState.connected ? 'Connected' : 'Not connected' }}</span></div>
            <p v-if="githubState.connected" class="connection-copy">GitHub App este instalată și poate accesa <strong>{{ githubRepositories.length || 'repository-urile permise' }}</strong> repository-uri, inclusiv private. Alegerea se face per site în wizard.</p>
            <p v-else-if="githubState.configured" class="connection-copy">GitHub App este configurată, dar nu există încă o instalare activă. Instalează App-ul și selectează repository-urile permise în GitHub.</p>
            <p v-else class="connection-copy">Configurează GitHub App pe serviciul NAS pentru acces securizat la repository-uri private, fără PAT.</p>
            <div class="connection-foot"><span class="muted-copy">{{ githubState.connected ? `${githubState.installations.length} installation${githubState.installations.length === 1 ? '' : 's'}` : (githubState.configuration_error || 'GitHub App not configured') }}</span><button class="button button-secondary" type="button" @click="activeNav = 'Settings'">{{ githubState.connected ? 'Manage GitHub App' : 'Configure GitHub App' }} <ArrowUpRight :size="14" /></button></div>
          </article>
        </section>

        <section v-if="activeNav === 'Activity'" class="activity-view">
          <div class="sites-management-header"><div><div class="eyebrow"><span class="eyebrow-pulse" /> AUTOMATION AUDIT</div><h1>Activity.</h1><p class="hero-copy">Every webhook verification and automatic deployment is recorded without storing payloads or secrets.</p></div><button class="button button-secondary" type="button" @click="loadAuditLogs"><RotateCw :size="15" :class="{ spin: auditLoading }" />Refresh</button></div>
          <section class="panel audit-panel"><div class="panel-heading"><div><div class="panel-kicker">RECENT EVENTS</div><h2>Automation timeline</h2></div><span class="muted-copy">{{ auditEntries.length }} recent events</span></div><div v-if="auditEntries.length" class="audit-list"><article v-for="entry in auditEntries" :key="entry.id" class="audit-row"><span class="audit-marker" :class="entry.action.includes('rejected') || entry.action.includes('failed') ? 'danger' : 'success'"><CircleX v-if="entry.action.includes('rejected') || entry.action.includes('failed')" :size="13" /><Check v-else :size="13" /></span><div class="audit-copy"><strong>{{ entry.action }}</strong><small>{{ entry.actor_type }}{{ entry.actor_id ? ` · ${entry.actor_id}` : '' }}{{ entry.entity_id ? ` · ${entry.entity_type}: ${entry.entity_id}` : '' }}</small><small v-if="entry.metadata?.provider || entry.metadata?.branch">{{ entry.metadata?.provider || '' }}{{ entry.metadata?.branch ? ` · ${entry.metadata.branch}` : '' }}{{ entry.metadata?.commit ? ` · ${String(entry.metadata.commit).slice(0, 7)}` : '' }}</small></div><time>{{ relativeTime(entry.created_at) }}</time></article></div><div v-else class="management-empty"><Activity :size="22" /><strong>No automation events yet</strong><span>Generate a site webhook and send a verified push to see the audit trail here.</span></div></section>
        </section>

        <section v-if="activeNav === 'Dashboard'" class="section-heading"><div><div class="panel-kicker">YOUR SURFACE</div><h2>Managed sites</h2></div><button class="text-button" type="button" @click="activeNav = 'Sites'">View all sites <ArrowUpRight :size="15" /></button></section>
        <section v-if="activeNav === 'Dashboard'" class="sites-grid">
          <article v-if="sites.length === 0" class="site-card empty-site-card">
            <Globe2 :size="20" />
            <strong>{{ sitesLoading ? 'Loading managed sites' : 'No managed sites yet' }}</strong>
            <span>Discover existing Web Station applications or add a site manually.</span>
          </article>
          <article v-for="site in sites" :key="site.id" class="site-card" :class="siteClass(site)" tabindex="0" role="button" @click="openSite(site)" @keydown.enter="openSite(site)">
            <div class="site-card-top"><span class="framework-mark"><Code2 :size="17" /></span><button class="more-button" type="button"><MoreHorizontal :size="17" /></button></div>
            <div class="site-domain">{{ site.hostname || site.name }}</div><div class="site-framework">{{ site.framework }}</div>
            <div class="site-status" :class="{ 'site-status-warning': site.status !== 'active' }"><span class="status-dot" />{{ displayStatus(site.status) }}<span class="site-status-time">{{ site.strategy }}</span></div>
            <div class="site-card-foot"><span><Globe2 :size="14" />{{ site.web_root }}</span><code>{{ site.project_root }}</code><button type="button" aria-label="Deploy site" :disabled="deployingSiteId === site.id || !site.repository || site.strategy !== 'atomic'" @click.stop="deploySite(site)"><Play :size="14" /></button></div>
          </article>
          <button class="site-card add-site-card" type="button" @click="openWizard"><span class="add-site-icon"><Plus :size="19" /></span><strong>Add site manually</strong><span>Configure URL, repository and synchronization</span></button>
        </section>

        <Transition name="toast">
          <div v-if="deployMessage || deployError" class="deployment-toast" :class="{ error: deployError }" role="status" aria-live="polite" @click="dismissDeploymentToast"><span class="deployment-toast-icon"><Check v-if="deployMessage" :size="16" /><CircleX v-else :size="16" /></span><span><strong>{{ deployError ? 'Deployment failed' : 'Deployment completed' }}</strong><small>{{ deployMessage || deployError }}</small></span><button type="button" aria-label="Close notification"><X :size="14" /></button></div>
        </Transition>
        <Transition name="live-deployment">
          <button v-if="deploymentNotice" class="live-deployment-notice" type="button" :class="{ failed: deploymentNotice.status === 'failed', completed: deploymentNotice.status === 'deployed' }" @click="openDeploymentNotice">
            <span class="live-deployment-icon"><Check v-if="deploymentNotice.status === 'deployed'" :size="16" /><CircleX v-else-if="deploymentNotice.status === 'failed'" :size="16" /><RotateCw v-else :size="16" class="spin" /></span>
            <span class="live-deployment-copy"><strong>{{ deploymentNotice.status === 'queued' ? 'Deployment queued' : (deploymentNotice.status === 'running' ? 'Deployment in progress' : (deploymentNotice.status === 'deployed' ? 'Deployment completed' : 'Deployment failed')) }}</strong><small>{{ deploymentNotice.siteName }} · {{ deploymentNotice.message }}</small><span class="live-deployment-progress"><i :style="{ width: `${deploymentNotice.progress}%` }" /></span><em>Click to open live deployment logs</em></span>
            <span class="live-deployment-close" aria-label="Close notification" @click.stop="dismissDeploymentNotice"><X :size="14" /></span>
          </button>
        </Transition>

        <section v-if="activeNav === 'Sites'" class="sites-management">
          <div class="sites-management-header"><div><div class="eyebrow"><span class="eyebrow-pulse" /> SITE CATALOG</div><h1>Managed sites.</h1><p class="hero-copy">Review imported Web Station applications, document roots and deployment readiness.</p></div><div class="hero-actions"><button class="button button-secondary" type="button" @click="loadSites"><RotateCw :size="15" />Refresh</button><button class="button button-secondary" type="button" @click="openDiscovery"><Globe2 :size="15" />Discover Web Station</button><button class="button button-primary" type="button" @click="openWizard"><Plus :size="15" />Add site</button></div></div>
          <div class="sites-management-summary"><span><strong>{{ sites.length }}</strong> managed sites</span><span><span class="status-dot" />{{ webStationLabel }}</span><span>Discovery adapter: read-only</span></div>
          <div v-if="sites.length" class="management-list">
            <article v-for="site in sites" :key="site.id" class="management-row" tabindex="0" role="button" @click="openSite(site)" @keydown.enter="openSite(site)">
              <button class="button button-secondary" type="button" :disabled="deployingSiteId === site.id || !site.repository || site.strategy !== 'atomic'" @click.stop="deploySite(site)">{{ deployingSiteId === site.id ? 'Deploying…' : 'Deploy' }} <Play :size="14" /></button>
              <span class="framework-mark"><Code2 :size="17" /></span><span class="management-copy"><strong>{{ site.hostname || site.name }}</strong><small>{{ frameworkLabel(site.framework) }} · {{ site.web_root }}</small><small>{{ site.project_root }}</small></span><span class="discovery-badge" :class="site.status === 'active' ? 'ready' : 'read_only'">{{ displayStatus(site.status) }}</span><button class="more-button" type="button" aria-label="Archive site" @click.stop="archiveSite(site)"><X :size="15" /></button>
            </article>
          </div>
          <div v-else class="management-empty"><Globe2 :size="24" /><strong>No sites are managed yet</strong><span>Discover a Web Station application or configure one manually.</span><div class="hero-actions"><button class="button button-secondary" type="button" @click="openDiscovery">Discover applications</button><button class="button button-primary" type="button" @click="openWizard">Add manually</button></div></div>
        </section>

        <section v-if="activeNav === 'SiteDetail' && selectedSite" class="site-detail-view">
          <div class="site-detail-header"><div><button class="text-button" type="button" @click="closeSiteDetail">← All sites</button><div class="eyebrow"><span class="eyebrow-pulse" /> SITE WORKSPACE</div><h1>{{ selectedSite.hostname || selectedSite.name }}.</h1><p class="hero-copy">{{ frameworkLabel(selectedSite.framework) }} · {{ selectedSite.project_root }}</p></div><div class="hero-actions"><a v-if="sitePublicURL(selectedSite).startsWith('http')" class="button button-secondary" :href="sitePublicURL(selectedSite)" target="_blank" rel="noreferrer"><Globe2 :size="15" />Visit</a><button class="button button-primary" type="button" :disabled="!selectedSite.repository || selectedSite.strategy !== 'atomic'" @click="deploySite(selectedSite)"><Play :size="15" />Deploy</button></div></div>
          <div class="site-tabs"><button type="button" :class="{ active: selectedSiteTab === 'Overview' }" @click="selectedSiteTab = 'Overview'">Overview</button><button type="button" :class="{ active: selectedSiteTab === 'Deployments' }" @click="selectedSiteTab = 'Deployments'">Deployments <span>{{ siteDeployments.length }}</span></button><button type="button" :class="{ active: selectedSiteTab === 'Repository' }" @click="openRepositoryTab">Repository</button><button type="button" :class="{ active: selectedSiteTab === 'Settings' }" @click="openSiteSettings">Settings</button></div>
          <template v-if="selectedSiteTab === 'Overview'">
            <div class="site-meta-grid"><article class="panel site-meta-card"><span>Framework</span><strong>{{ frameworkLabel(selectedSite.framework) }}</strong></article><article class="panel site-meta-card"><span>PHP</span><strong>{{ siteRuntime(selectedSite, 'php_version', '8.5') }}</strong></article><article class="panel site-meta-card"><span>HTTP server</span><strong>{{ siteRuntime(selectedSite, 'http_server', 'Unknown') }}</strong></article><article class="panel site-meta-card"><span>Public IP</span><button type="button" @click="copyPublicIP(selectedSite)">{{ siteRuntime(selectedSite, 'public_ip', 'Not detected') }}</button></article><article class="panel site-meta-card"><span>Public Web</span><a :href="sitePublicURL(selectedSite)" target="_blank" rel="noreferrer">{{ sitePublicURL(selectedSite) }}</a></article><article class="panel site-meta-card"><span>Created</span><strong>{{ createdLabel(selectedSite.created_at) }}</strong></article></div>
            <section class="release-history-panel panel"><div class="release-history-heading"><div><div class="panel-kicker">RELEASE MANAGEMENT</div><h2>Release history</h2><p>Activated releases are retained for safe rollback. Application files can be restored without reversing database migrations.</p></div><span class="release-retention-pill">{{ selectedSite.deployment_retention ?? 4 }} retained</span></div><div v-if="releaseMessage" class="discovery-success"><Check :size="15" />{{ releaseMessage }}</div><div v-if="releaseError" class="discovery-error"><CircleAlert :size="15" />{{ releaseError }}</div><div v-if="releasesLoading" class="detail-loading"><RotateCw :size="16" class="spin" /> Loading releases…</div><div v-else-if="siteReleases.length" class="release-timeline"><article v-for="(release, index) in siteReleases" :key="release.id" class="release-timeline-item" :class="{ current: release.active }"><span class="release-timeline-marker"><Check v-if="release.active" :size="12" /><CircleDot v-else :size="12" /></span><div class="release-timeline-copy"><div class="release-timeline-title"><strong>{{ release.active ? 'Current' : (index === 1 ? 'Previous' : 'Release') }}</strong><code>{{ release.commit_sha ? release.commit_sha.slice(0, 7) : release.id }}</code><span class="release-health" :class="release.health_status || 'unknown'">{{ release.health_status || 'not checked' }}</span></div><p>{{ release.commit_message || 'Release without commit message' }}</p><small>{{ release.branch || '—' }} · {{ relativeTime(release.activated_at || release.created_at) }} · {{ release.release_path }}</small></div><button v-if="!release.active" class="button button-secondary release-rollback-button" type="button" :disabled="rollbackReleaseID !== ''" @click="rollbackRelease(release)"><RotateCw v-if="rollbackReleaseID === release.id" :size="13" class="spin" /><span v-else>Rollback here</span></button></article></div><div v-else class="management-empty release-empty"><Database :size="21" /><strong>No releases recorded yet</strong><span>The first successful Atomic deployment will appear here with a rollback target.</span></div><div class="release-warning"><CircleAlert :size="14" /><span>Rollback restores application files only. Database migrations are not reversed automatically.</span></div></section>
            <div class="detail-section-heading"><div><div class="panel-kicker">SOURCE HISTORY</div><h2>Commits</h2></div><span class="muted-copy">{{ selectedSite.repository?.branch || 'main' }}</span></div>
            <div class="commit-legend" aria-label="Commit status legend"><span><span class="commit-state deployed"><Check :size="11" /></span>Deployed directly</span><span><span class="commit-state included"><CircleDot :size="11" /></span>Included in a newer deployed commit</span><span><span class="commit-state pending"><CircleDot :size="11" /></span>Not deployed</span><span><span class="commit-state failed"><CircleX :size="11" /></span>Deployment failed</span></div>
            <div v-if="siteDetailLoading" class="detail-loading"><RotateCw :size="16" class="spin" /> Loading repository history…</div>
            <div v-else-if="siteCommits.length" class="commit-table"><div v-for="commit in siteCommits" :key="commit.sha" class="commit-row" :class="commit.status"><span class="commit-state" :class="commit.status" v-if="commit.deployed"><Check :size="13" /></span><span class="commit-state included" v-else-if="commit.included_in_deployed"><CircleDot :size="13" /></span><span class="commit-state failed" v-else-if="commit.status === 'failed'"><CircleX :size="13" /></span><span class="commit-state pending" v-else><CircleDot :size="13" /></span><code>{{ commit.sha.slice(0, 7) }}</code><span class="commit-message"><strong>{{ commit.message.split('\n')[0] }}</strong><small>{{ commit.author || 'GitHub' }} · {{ relativeTime(commit.created_at) }}</small></span><span class="commit-status">{{ commit.deployed ? 'Deployed directly' : (commit.included_in_deployed ? 'Included in newer deploy' : (commit.status === 'failed' ? 'Failed' : 'Not deployed')) }}</span><button class="button button-secondary commit-deploy" type="button" :disabled="deployingCommitSha !== '' || commit.deployed || commit.included_in_deployed || selectedSite.strategy !== 'atomic'" @click="deployCommit(commit)"><RotateCw v-if="deployingCommitSha === commit.sha" :size="13" class="spin" /><span v-else>Deploy</span></button></div></div>
            <div v-else class="management-empty"><GitBranch :size="22" /><strong>No commits available</strong><span>Connect GitHub and configure a repository for this site to see commit history.</span></div>
          </template>
          <template v-else-if="selectedSiteTab === 'Settings'">
            <div class="site-settings-layout">
              <section class="site-settings-section"><div class="site-settings-heading"><div><div class="panel-kicker">DEPLOYMENT</div><h2>Deployment</h2></div><span class="muted-copy">{{ selectedSite.repository?.branch || 'No branch selected' }}</span></div>
                <label class="settings-toggle-row"><span><strong>Push to deploy</strong><small>Deploy automatically after every push to the selected branch.</small></span><input v-model="siteSettingsForm.push_to_deploy" type="checkbox" /></label>
                <label class="site-field"><span>Deploy script</span><div class="deploy-script-editor-shell"><div class="deploy-script-editor-toolbar"><span><TerminalSquare :size="13" /> deploy.sh</span><small>Shell · {{ deployScriptLineCount }} lines</small></div><div ref="deployScriptEditor" class="deploy-script-editor" aria-label="Deployment script editor" /></div><small>Scriptul rulează ca utilizatorul pachetului. Variabile disponibile: <code>$PROJECT_ROOT</code>, <code>$CURRENT_DIR</code>, <code>$RELEASE_DIR</code>, <code>$FORGE_RELEASE_DIRECTORY</code>, <code>$FORGE_PHP</code>, <code>$FORGE_COMPOSER</code>, <code>$FORGE_NPM</code>, <code>$FORGE_NODE</code>, <code>$WEB_ROOT</code>, <code>$RELEASE_ID</code> și <code>$COMMIT_SHA</code>.</small></label>
                <label class="site-field"><span>Deployment retention</span><input v-model.number="siteSettingsForm.deployment_retention" type="number" min="0" max="100" /><small>Configure the number of previous releases to retain after each successful deployment. Default: 4. The current release is always protected.</small></label>
                <label class="site-field"><span>Health check URL</span><input v-model="siteSettingsForm.health_check_url" type="url" placeholder="https://example.com/health" /><small>Optional HTTP(S) URL checked after activation. A failed check triggers automatic rollback to the previous release.</small></label>
                <div class="site-field"><span>Shared directories</span><div class="tag-editor"><span v-for="directory in siteSettingsForm.shared_directories" :key="directory" class="tag-badge shared-directory-badge">{{ directory }}<button type="button" :aria-label="`Remove shared directory ${directory}`" @click="removeSharedDirectory(directory)"><X :size="11" /></button></span><input v-model="sharedDirectoryDraft" type="text" placeholder="storage/ and press Enter" @keydown="onSharedDirectoryKeydown" /></div><small>Directories persist in <code>.zion/shared</code> across releases and are linked into every activated release. Use relative paths such as <code>storage</code> or <code>uploads</code>.</small></div>
                <div class="automation-card">
                  <div class="automation-card-heading"><span class="settings-icon"><Webhook :size="17" /></span><div><strong>Push automation</strong><small>Webhook direct pentru GitHub sau GitLab. Deploy-ul pornește numai după validarea semnăturii, repository-ului și branch-ului.</small></div><span v-if="siteWebhook.configured" class="connection-badge connected"><span class="status-dot" />{{ siteWebhook.provider }} ready</span><span v-else class="connection-badge"><span class="status-dot" />Not configured</span></div>
                  <div v-if="siteWebhookLoading" class="detail-loading"><RotateCw :size="15" class="spin" /> Loading webhook configuration…</div>
                  <template v-else>
                    <div v-if="siteWebhook.configured" class="webhook-endpoint-grid"><label class="site-field"><span>Webhook endpoint</span><div class="copy-input"><input :value="siteWebhook.endpoint" readonly @click="copyWebhookValue(siteWebhook.endpoint || '')" /><button type="button" title="Copy webhook endpoint" @click="copyWebhookValue(siteWebhook.endpoint || '')"><ArrowUpRight :size="13" /></button></div></label><label v-if="siteWebhookSecret" class="site-field"><span>Secret — shown once</span><div class="copy-input"><input :value="siteWebhookSecret" readonly @click="copyWebhookValue(siteWebhookSecret)" /><button type="button" title="Copy webhook secret" @click="copyWebhookValue(siteWebhookSecret)"><ArrowUpRight :size="13" /></button></div></label></div>
                    <div v-if="siteWebhookSecret" class="webhook-secret-warning"><CircleAlert :size="14" /><span>Salvează secretul în setarea Webhook din GitHub/GitLab. Nu este stocat în SPK și nu va mai fi afișat după ce părăsești această pagină.</span></div>
                    <div v-if="siteWebhook.last_delivery_at || siteWebhook.last_error" class="webhook-health-line"><span><strong>Last delivery</strong>{{ siteWebhook.last_delivery_at ? relativeTime(siteWebhook.last_delivery_at) : 'No valid delivery yet' }}</span><span v-if="siteWebhook.last_error" class="webhook-error"><CircleX :size="13" />{{ siteWebhook.last_error }}</span></div>
                    <p v-if="!siteWebhook.configured" class="settings-explanation">Generează un endpoint per site, apoi în provider alege evenimentul <strong>Push</strong>. Secretul este criptat la runtime pe NAS; Release Station aplică politica <strong>latest</strong>, păstrând doar ultimul commit care încă așteaptă să fie executat.</p>
                    <div class="settings-actions"><span class="muted-copy">{{ siteWebhook.configured ? 'Rotate invalidates the previous secret.' : 'No external webhook is active.' }}</span><button class="button button-secondary" type="button" :disabled="siteWebhookRotating" @click="rotateSiteWebhook">{{ siteWebhookRotating ? 'Generating…' : (siteWebhook.configured ? 'Rotate credentials' : 'Generate webhook') }} <Webhook :size="14" /></button></div>
                  </template>
                  <div v-if="siteWebhookError" class="discovery-error"><CircleAlert :size="14" />{{ siteWebhookError }}</div><div v-if="siteWebhookMessage" class="discovery-success"><Check :size="14" />{{ siteWebhookMessage }}</div>
                </div>
              </section>
              <section class="site-settings-section"><div class="site-settings-heading"><div><div class="panel-kicker">GENERAL</div><h2>General</h2></div></div>
                <label class="site-field"><span>Framework</span><select v-model="siteSettingsForm.framework"><option v-for="option in frameworkSettingOptions" :key="option.value" :value="option.value">{{ option.label }}</option></select><small>The framework used by the installed application. Changing the framework does not modify the Nginx/apache configuration.</small></label>
                <label v-if="siteSettingsForm.framework === 'custom'" class="site-field"><span>Custom framework name</span><input v-model="siteSettingsForm.custom_framework" maxlength="100" type="text" placeholder="e.g. Astro SSR" /><small>{{ siteSettingsForm.custom_framework.length }}/100 characters</small></label>
                <div class="site-field"><span>Tags</span><div class="tag-editor"><span v-for="tag in siteSettingsForm.tags" :key="tag" class="tag-badge">{{ tag }}<button type="button" :aria-label="`Remove ${tag}`" @click="removeSiteTag(tag)"><X :size="11" /></button></span><input v-model="siteTagDraft" type="text" placeholder="Type a tag and press Enter" @keydown="onSiteTagKeydown" /></div><small>Add as many tags as needed for organization and later filtering.</small></div>
                <label class="site-field color-field"><span>Color</span><div class="color-control"><input v-model="siteSettingsForm.color" type="color" /><code>{{ siteSettingsForm.color }}</code></div><small>Used later to identify site cards visually.</small></label>
              </section>
            </div>
            <section class="site-settings-section directories-section"><div class="site-settings-heading"><div><div class="panel-kicker">DIRECTORIES</div><h2>Directories</h2></div><span class="muted-copy">Read-only</span></div><div class="directory-grid"><label class="site-field"><span>Root directory</span><div class="copy-input"><input :value="selectedSite.project_root" readonly @click="copySiteDirectory(selectedSite.project_root)" /><button type="button" title="Copy root directory" @click="copySiteDirectory(selectedSite.project_root)"><ArrowUpRight :size="13" /></button></div></label><label class="site-field"><span>Web directory</span><div class="copy-input"><input :value="selectedSite.web_root" readonly @click="copySiteDirectory(selectedSite.web_root)" /><button type="button" title="Copy web directory" @click="copySiteDirectory(selectedSite.web_root)"><ArrowUpRight :size="13" /></button></div></label></div></section>
            <div v-if="siteSettingsError" class="discovery-error"><CircleAlert :size="16" />{{ siteSettingsError }}</div><div v-if="siteSettingsMessage" class="discovery-success"><Check :size="16" />{{ siteSettingsMessage }}</div><div class="settings-actions site-settings-actions"><span /><button class="button button-primary" type="button" :disabled="siteSettingsSaving" @click="saveSiteSettings">{{ siteSettingsSaving ? 'Saving…' : 'Save site settings' }} <Check :size="14" /></button></div>
          </template>
          <template v-else-if="selectedSiteTab === 'Repository'">
            <div class="detail-section-heading"><div><div class="panel-kicker">SOURCE CONFIGURATION</div><h2>Repository</h2></div><span class="connection-badge" :class="{ connected: !!selectedSite.repository }"><span class="status-dot" />{{ selectedSite.repository ? 'Configured' : 'Not configured' }}</span></div>
            <div class="repository-editor panel">
              <div class="wizard-intro"><GitBranch :size="20" /><div><strong>Asociază sursa site-ului</strong><span>Alege un repository din GitHub Connector sau editează URL-ul și branch-ul manual. Această configurație este folosită la citirea commit-urilor și la deploy.</span></div></div>
              <div class="form-grid"><label><span>Provider</span><select v-model="repositoryForm.provider"><option value="github">GitHub</option><option value="gitlab">GitLab</option><option value="bitbucket">Bitbucket</option><option value="generic">Other Git server</option></select></label><label><span>Branch</span><select v-if="repositoryForm.provider === 'github'" v-model="repositoryForm.branch" :disabled="!repositoryForm.github_full_name || githubBranchesLoading"><option v-if="!repositoryForm.github_full_name" value="">Select repository first</option><option v-else-if="githubBranchesLoading" value="">Loading branches…</option><option v-for="branch in githubBranches" :key="branch" :value="branch">{{ branch }}</option></select><input v-else v-model="repositoryForm.branch" type="text" placeholder="main" /></label></div>
              <label><span>Repository from connected GitHub</span><select :value="repositoryForm.github_full_name" @change="selectSiteGithubRepository(($event.target as HTMLSelectElement).value)"><option value="">Select a repository or use the URL below</option><optgroup v-for="group in githubRepositoryGroups" :key="group.account" :label="`@${group.account}`"><option v-for="repository in group.repositories" :key="`${repository.installation_id}:${repository.id}`" :value="repository.full_name">{{ repository.name }}{{ repository.private ? ' · private' : '' }}</option></optgroup></select><small v-if="githubLoading">Loading repositories…</small><small v-else-if="!githubState.connected">GitHub nu este conectat. Îl poți conecta din Settings.</small><button v-if="githubState.connected" class="text-button" type="button" @click="installGithubApp">Add another GitHub account <ArrowUpRight :size="13" /></button></label>
              <label><span>Repository clone URL</span><input v-model="repositoryForm.clone_url" type="text" placeholder="https://github.com/owner/project.git" /><small>Repository-ul poate fi public sau privat. Pentru private, accesul trebuie acordat în instalarea GitHub Connector.</small></label><div v-if="githubBranchesError" class="wizard-hint warning"><CircleAlert :size="15" /><span>{{ githubBranchesError }}</span></div>
              <div class="strategy-grid"><label class="strategy-card" :class="{ selected: repositoryForm.strategy === 'in_place' }"><input v-model="repositoryForm.strategy" type="radio" value="in_place" /><span class="strategy-title">In-place <em>direct</em></span><span>Actualizează fișierele direct în document root-ul Web Station. Este rapid și păstrează datele locale, dar un deploy eșuat poate lăsa modificări parțiale și rollback-ul este manual.</span></label><label class="strategy-card" :class="{ selected: repositoryForm.strategy === 'atomic' }"><input v-model="repositoryForm.strategy" type="radio" value="atomic" /><span class="strategy-title">Atomic releases <em>safer rollback</em></span><span>Pregătește separat fiecare release în <code>.zion/releases</code>, îl expune prin <code>.current</code>, rulează scriptul de build/deploy, apoi publică rezultatul în document root. Release-urile anterioare rămân disponibile pentru rollback.</span></label></div>
              <div class="wizard-hint"><CircleHelp :size="15" /><span><strong>Diferența:</strong> In-place scrie direct în document root-ul site-ului. Atomic construiește și verifică noul release în <code>{{ selectedSite.project_root }}/.current</code>, rulează comenzile configurate (de exemplu Composer, migrations sau npm), apoi copiază rezultatul în document root și păstrează release-ul vechi pentru revenire.</span></div>
              <div class="repository-transport panel"><div class="transport-heading"><div><div class="panel-kicker">GIT TRANSPORT</div><strong>Deploy key și verificare SSH</strong><span>Cheia privată rămâne criptată pe NAS. Release Station expune doar cheia publică și verifică host-ul prin <code>known_hosts</code>.</span></div><span class="connection-badge" :class="{ connected: gitTransportState === 'success' }"><span class="status-dot" />{{ gitTransportState === 'success' ? 'Verified' : 'Not tested' }}</span></div><div class="transport-actions"><button class="button button-secondary" type="button" :disabled="deployKeyLoading" @click="generateDeployKey">{{ deployKeyLoading ? 'Generating…' : 'Generate deploy key' }}</button><button class="button button-secondary" type="button" :disabled="gitTransportState === 'testing' || !repositoryForm.clone_url || !repositoryForm.branch" @click="testGitTransport">{{ gitTransportState === 'testing' ? 'Testing…' : 'Test Git connection' }}</button></div><label v-if="deployPublicKey" class="transport-key"><span>Public key — add it in the target repository’s own settings, not in your GitHub account settings.</span><div class="copy-input"><input :value="deployPublicKey" readonly @click="copySiteDirectory(deployPublicKey)" /><button type="button" title="Copy public key" @click="copySiteDirectory(deployPublicKey)"><ArrowUpRight :size="13" /></button></div><small>On GitHub: open the repository → <strong>Settings → Deploy keys → Add deploy key</strong>. Deploy keys are configured per repository, so repeat this for each repository Release Station must access.</small><small v-if="deployKeyFingerprint">Fingerprint: <code>{{ deployKeyFingerprint }}</code></small></label><div v-if="gitTransportMessage" class="wizard-hint" :class="{ warning: gitTransportState === 'error' }"><CircleAlert v-if="gitTransportState === 'error'" :size="15" /><Check v-else :size="15" /><span>{{ gitTransportMessage }}</span></div></div>
              <div v-if="repositoryError" class="discovery-error"><CircleAlert :size="16" />{{ repositoryError }}</div><div v-if="repositoryMessage" class="discovery-success"><Check :size="16" />{{ repositoryMessage }}</div>
              <div class="settings-actions"><button class="button button-secondary" type="button" :disabled="repositorySaving || !selectedSite.repository" @click="disconnectSiteRepository">Remove repository</button><span /><button class="button button-primary" type="button" :disabled="repositorySaving" @click="saveSiteRepository">{{ repositorySaving ? 'Saving…' : 'Save repository' }} <Check :size="14" /></button></div>
            </div>
          </template>
          <template v-else>
            <div class="deployments-toolbar"><div><div class="panel-kicker">DEPLOYMENT HISTORY</div><h2>Deployments</h2></div><input v-model="deploymentSearch" type="search" placeholder="Search commit name…" @change="sitePage = 1; loadSiteHistory()" /></div>
            <div v-if="siteDeployments.length" class="deployment-history-list"><button v-for="deployment in siteDeployments" :key="deployment.id" class="deployment-history-row" type="button" @click="openDeploymentDetails(deployment)"><span class="commit-state" :class="deployment.status"><Check v-if="deployment.status === 'deployed'" :size="13" /><RotateCw v-else-if="deployment.status === 'running' || deployment.status === 'queued'" :size="13" class="spin" /><CircleX v-else :size="13" /></span><code>{{ deployment.commit_sha ? deployment.commit_sha.slice(0, 7) : 'unknown' }}</code><span class="commit-message"><strong>{{ deployment.commit_message || 'Deployment without commit metadata' }}</strong><small>{{ deployment.branch || '—' }} · {{ relativeTime(deployment.created_at) }} · {{ durationLabel(deployment.duration_ms) }}</small></span><span class="discovery-badge" :class="deployment.status">{{ deployment.status }}</span><span class="row-arrow">→</span></button></div><div v-else class="management-empty"><Clock3 :size="22" /><strong>No deployments yet</strong><span>Deploy a commit from Overview to build the deployment history.</span></div>
            <div class="pagination"><button type="button" :disabled="sitePage <= 1" @click="setDeploymentPage(sitePage - 1)">Previous</button><span>Page {{ sitePage }} of {{ siteTotalPages }}</span><button type="button" :disabled="sitePage >= siteTotalPages" @click="setDeploymentPage(sitePage + 1)">Next</button></div>
          </template>
        </section>

        <section v-if="activeNav === 'DeploymentDetail' && selectedSite && selectedDeployment" class="deployment-detail-view">
          <div class="deployment-detail-header"><div><button class="text-button" type="button" @click="backToSiteTab('Deployments')">← Back to deployments</button><h1>Deployment details <span>·</span> <code>{{ selectedDeployment.commit_sha ? selectedDeployment.commit_sha.slice(0, 7) : 'unknown' }}</code></h1><div class="deployment-detail-meta"><span class="commit-state" :class="selectedDeployment.status"><Check v-if="selectedDeployment.status === 'deployed'" :size="13" /><RotateCw v-else-if="selectedDeployment.status === 'running' || selectedDeployment.status === 'queued'" :size="13" class="spin" /><CircleX v-else :size="13" /></span><strong>{{ selectedDeployment.status }}</strong><span>{{ selectedDeployment.branch }}</span><span>·</span><span>{{ selectedDeployment.commit_message || 'No commit message' }}</span><span>·</span><span>{{ relativeTime(selectedDeployment.finished_at || selectedDeployment.created_at) }}</span><span>·</span><span>{{ durationLabel(selectedDeployment.duration_ms) }}</span></div></div><div class="hero-actions"><a v-if="sitePublicURL(selectedSite).startsWith('http')" class="button button-secondary" :href="sitePublicURL(selectedSite)" target="_blank" rel="noreferrer"><Globe2 :size="15" />Visit</a><button class="button button-secondary" type="button" @click="logsExpanded = !logsExpanded">{{ logsExpanded ? 'Collapse all' : 'Expand all' }}</button></div></div>
          <div class="detail-facts"><span><strong>Method</strong>{{ selectedDeployment.deployment_method || selectedDeployment.trigger_type || 'manual' }}</span><span><strong>Branch</strong>{{ selectedDeployment.branch || '—' }}</span><span><strong>Commit</strong>{{ selectedDeployment.commit_sha || '—' }}</span><span><strong>Started</strong>{{ selectedDeployment.started_at || '—' }}</span></div>
          <div v-if="selectedDeployment.steps?.length" class="pipeline-steps"><div v-for="step in selectedDeployment.steps" :key="step.id" class="pipeline-step-row"><span class="commit-state" :class="step.status"><Check v-if="step.status === 'completed'" :size="12" /><RotateCw v-else-if="step.status === 'running'" :size="12" class="spin" /><CircleX v-else :size="12" /></span><strong>{{ step.name }}</strong><span>{{ step.status }}</span><small>{{ durationLabel(step.duration_ms) }}</small></div></div>
          <details :open="logsExpanded" class="log-panel"><summary><span class="commit-state" :class="selectedDeployment.status"><Check v-if="selectedDeployment.status === 'deployed'" :size="13" /><RotateCw v-else-if="selectedDeployment.status === 'running' || selectedDeployment.status === 'queued'" :size="13" class="spin" /><CircleX v-else :size="13" /></span><strong>Build logs</strong><span>{{ durationLabel(selectedDeployment.duration_ms) }}</span></summary><pre>{{ selectedDeployment.build_log || 'No build output was captured.' }}</pre></details>
          <details :open="logsExpanded" class="log-panel"><summary><span class="commit-state" :class="selectedDeployment.status"><Check v-if="selectedDeployment.status === 'deployed'" :size="13" /><RotateCw v-else-if="selectedDeployment.status === 'running' || selectedDeployment.status === 'queued'" :size="13" class="spin" /><CircleX v-else :size="13" /></span><strong>Deployment logs</strong><span>{{ durationLabel(selectedDeployment.duration_ms) }}</span></summary><pre>{{ selectedDeployment.deployment_log || 'No deployment output was captured.' }}</pre></details>
        </section>

        <section v-if="activeNav === 'Settings'" class="settings-view">
          <div class="sites-management-header"><div><div class="eyebrow"><span class="eyebrow-pulse" /> WORKSPACE SETTINGS</div><h1>Settings.</h1><p class="hero-copy">Connect the services Release Station uses to discover repositories and deploy sites.</p></div></div>
          <article class="panel settings-card system-checks-card">
            <div class="settings-card-heading"><span class="settings-icon"><ServerCog :size="18" /></span><div><div class="panel-kicker">SYSTEM OVERVIEW</div><h2>Deployment toolchain checks</h2><p>Selectează ce vrei să verifici live pe NAS și să afișezi în Dashboard. O verificare roșie înseamnă că binarul nu a fost găsit de serviciul Release Station.</p></div></div>
            <div v-if="systemChecksLoading && !systemChecks.length" class="management-empty"><RotateCw :size="18" class="spin" /><span>Loading available checks…</span></div>
            <div v-else class="system-check-settings-grid"><label v-for="check in systemChecks" :key="check.id" class="system-check-setting"><input v-model="check.enabled" type="checkbox" /><span><strong>{{ check.label }}</strong><small>{{ check.description }}</small><code>{{ check.command }}</code></span></label></div>
            <div v-if="systemChecksError" class="discovery-error"><CircleAlert :size="16" />{{ systemChecksError }}</div><div v-if="systemChecksMessage" class="discovery-success"><Check :size="16" />{{ systemChecksMessage }}</div>
            <div class="settings-actions"><span class="muted-copy">{{ systemChecks.filter((check) => check.enabled).length }} active checks</span><button class="button button-primary" type="button" :disabled="systemChecksSaving || systemChecksLoading" @click="saveSystemChecks">{{ systemChecksSaving ? 'Saving…' : 'Save checks' }} <Check :size="14" /></button></div>
            <div class="settings-note"><CircleHelp :size="15" /><span>Instalează tool-urile din Package Center când există pachet DSM. Pentru verificarea exactă, folosește SSH și comenzile afișate în articolul Read more al fiecărui card.</span></div>
          </article>
          <article v-if="githubState.mode === 'managed'" class="panel settings-card">
            <div class="settings-card-heading"><span class="settings-icon"><GitBranch :size="18" /></span><div><div class="panel-kicker">SOURCE CONTROL CONNECTOR</div><h2>GitHub</h2><p>Conectează GitHub fără să creezi o aplicație proprie și fără să încarci cheia PEM pe NAS.</p></div><span class="connection-badge" :class="{ connected: githubState.connected }"><span class="status-dot" />{{ githubState.connected ? 'Connected' : 'Not connected' }}</span></div>
            <div class="settings-form"><div class="connector-status"><span class="status-dot" :class="{ 'status-dot-warning': !githubState.connected }" /><strong>{{ githubState.connected ? 'GitHub connected through Synology Connector' : 'Connect GitHub to continue' }}</strong><small v-if="githubState.account_login">Account: {{ githubState.account_login }}</small><small v-else-if="githubState.configuration_error">{{ githubState.configuration_error }}</small><small>Webhook: {{ githubState.webhook_configured ? 'configured' : 'not configured' }} · accepted events: {{ githubState.webhook_accepted_events || 0 }}</small><small v-if="githubState.webhook_endpoint">Endpoint: {{ githubState.webhook_endpoint }}</small></div><p class="settings-explanation">Apasă butonul, autentifică-te pe GitHub și instalează aplicația „Synology Connector” în contul sau organizația ta. Selectezi exact repository-urile private accesibile. Pentru push-to-deploy, configurează în aceeași aplicație GitHub webhook-ul Push către endpointul afișat, cu același secret setat în Zion Connector ca <code>CONNECTOR_GITHUB_WEBHOOK_SECRET</code>. Secretul nu ajunge în SPK.</p></div>
            <div v-if="githubError" class="discovery-error"><CircleAlert :size="16" />{{ githubError }}</div><div v-if="githubMessage" class="discovery-success"><Check :size="16" />{{ githubMessage }}</div>
            <div class="settings-actions"><span /><button class="button button-secondary" type="button" @click="loadGithubStatus">Refresh status</button><button class="button button-primary" type="button" @click="installGithubApp" :disabled="githubConnectState !== 'idle'">{{ githubConnectState === 'waiting' ? 'Waiting for GitHub…' : (githubState.connected ? 'Manage GitHub' : 'Connect GitHub') }} <ArrowUpRight :size="14" /></button></div>
            <div class="settings-note"><CircleHelp :size="15" /><span>Acesta este modul recomandat pentru clienții Release Station: te conectezi prin aplicația GitHub „Synology Connector”, instalată în contul fiecărui client. Nu este nevoie de App ID, slug sau fișier PEM pe NAS.</span></div>
            <div v-if="githubState.installations.length" class="installation-list"><div v-for="installation in githubState.installations" :key="installation.github_installation_id" class="installation-row"><GitBranch :size="15" /><span><strong>{{ installation.account_login }}</strong><small>{{ installation.account_type }} · {{ installation.repository_selection }} repositories · installation {{ installation.github_installation_id }}</small></span><button class="button button-secondary" type="button" @click="loadGithubRepositories">Refresh repositories</button></div></div>
          </article>
          <article v-if="githubState.mode !== 'managed'" class="panel settings-card">
            <div class="settings-card-heading"><span class="settings-icon"><GitBranch :size="18" /></span><div><div class="panel-kicker">SOURCE CONTROL CONNECTOR</div><h2>GitHub App</h2><p>Instalează App-ul GitHub și selectează explicit unul sau mai multe repository-uri, inclusiv private.</p></div><span class="connection-badge" :class="{ connected: githubState.connected }"><span class="status-dot" />{{ githubState.connected ? 'Connected' : 'Not connected' }}</span></div>
            <div class="settings-form"><div class="connector-status"><span class="status-dot" :class="{ 'status-dot-warning': !githubState.configured }" /><strong>{{ githubState.configured ? 'App credentials detected on NAS' : 'App credentials are not configured' }}</strong><small v-if="githubState.configuration_error">{{ githubState.configuration_error }}</small><small v-else>App: {{ githubState.app_slug }}</small></div><p class="settings-explanation">Private repository access is granted in GitHub during installation. Release Station receives only short-lived installation tokens and never stores a PAT.</p></div>
            <div v-if="githubError" class="discovery-error"><CircleAlert :size="16" />{{ githubError }}</div><div v-if="githubMessage" class="discovery-success"><Check :size="16" />{{ githubMessage }}</div>
            <div class="settings-actions"><span /><button class="button button-secondary" type="button" @click="loadGithubStatus">Refresh status</button><button v-if="githubState.configured" class="button button-primary" type="button" @click="installGithubApp">Install / manage GitHub App <ArrowUpRight :size="14" /></button></div>
            <div class="settings-note"><CircleHelp :size="15" /><span>Setează Setup URL-ul GitHub App la <code>{{ githubState.setup_url || '/releasestation/api/v1/integrations/github/setup' }}</code>, apoi folosește butonul de mai sus. După instalare, repository-urile private selectate în GitHub apar în wizard.</span></div>
            <div v-if="githubState.installations.length" class="installation-list"><div v-for="installation in githubState.installations" :key="installation.github_installation_id" class="installation-row"><GitBranch :size="15" /><span><strong>{{ installation.account_login }}</strong><small>{{ installation.account_type }} · {{ installation.repository_selection }} repositories · installation {{ installation.github_installation_id }}</small></span><button class="button button-secondary" type="button" @click="loadGithubRepositories">Refresh repositories</button></div></div>
          </article>
        </section>

        <section v-if="activeNav === 'Help'" class="help-view">
          <div class="sites-management-header"><div><div class="eyebrow"><span class="eyebrow-pulse" /> DOCUMENTATION</div><h1>Help center.</h1><p class="hero-copy">Un ghid scurt pentru configurarea Release Station și remedierea verificărilor roșii din System Overview.</p></div><div class="hero-actions"><button class="button button-secondary" type="button" @click="activeNav = 'Dashboard'"><LayoutDashboard :size="15" />Back to dashboard</button></div></div>
          <div class="help-layout">
            <aside class="panel help-navigation"><button type="button" :class="{ active: helpTopic === 'intro' }" @click="helpTopic = 'intro'"><LifeBuoy :size="15" />Getting started</button><div class="help-nav-label">SYSTEM OVERVIEW</div><button v-for="item in systemItems" :key="item.label" type="button" :class="{ active: helpTopic === item.label }" @click="helpTopic = item.label"><component :is="item.icon" :size="15" />{{ item.label }}<span class="help-nav-state" :class="item.state">{{ item.state === 'ready' ? 'OK' : 'FIX' }}</span></button></aside>
          <article v-if="helpTopic === 'intro'" class="panel help-article"><div class="panel-kicker">ZION RELEASE STATION</div><h2>Deploy sites from Synology with confidence</h2><p>Release Station descoperă site-uri Web Station, le conectează la repository-uri GitHub și păstrează istoricul, starea release-urilor și logurile într-un singur control plane local.</p><div class="help-card-grid"><div><GitBranch :size="17" /><strong>Connect source control</strong><span>Conectează GitHub din Settings, aprobă repository-urile private, apoi selectează repository-ul și branch-ul pentru fiecare site.</span></div><div><Globe2 :size="17" /><strong>Configure a site</strong><span>Importă un site Web Station sau adaugă-l manual. Taburile Repository și Settings păstrează configurația deployment-ului.</span></div><div><UploadCloud :size="17" /><strong>Deploy and observe</strong><span>Alege Atomic releases pentru activare cu rollback, pornește deployment-ul și verifică logurile capturate.</span></div></div><div class="help-callout"><CircleHelp :size="16" /><span>Un card roșu din System Overview nu înseamnă că datele site-ului sunt pierdute. Deschide articolul lui din meniul din stânga pentru pașii de remediere.</span></div><div class="help-callout"><TerminalSquare :size="16" /><span>Atomic deploy pregătește release-ul, îl leagă în <code>.current</code>, apoi rulează scriptul din Settings. Document root-ul Web Station este chiar project root-ul site-ului, iar scriptul implicit publică acolo conținutul pregătit, păstrând directoarele interne `.zion` și `.current`. Poți înlocui scriptul cu propriile comenzi Composer, migrations sau npm; verifică tool-urile necesare în Settings → Deployment toolchain checks.</span></div></article>
            <article v-else class="panel help-article"><div class="panel-kicker">SYSTEM OVERVIEW / {{ helpTopic }}</div><h2>{{ helpArticles[helpTopic]?.title || helpTopic }}</h2><p>{{ helpArticles[helpTopic]?.summary || systemItem(helpTopic)?.description || 'Verificarea acestui serviciu este furnizată de API-ul local.' }}</p><div class="help-status-line"><span class="status-dot" :class="{ 'status-dot-warning': systemItem(helpTopic)?.state !== 'ready' }" /><strong>{{ systemItem(helpTopic)?.state === 'ready' ? 'Verification passed' : 'Action required' }}</strong><span>{{ systemItem(helpTopic)?.detail || 'No live detail available.' }}</span></div><h3>How to fix it</h3><ol class="help-steps"><li v-for="step in helpSteps(helpTopic)" :key="step">{{ step }}</li></ol><div class="help-callout"><CircleHelp :size="16" /><span>După remediere, revino pe Dashboard și apasă Refresh în System Overview pentru o verificare live.</span></div></article>
          </div>
        </section>

        <footer v-if="activeNav === 'Dashboard'" class="footer-note"><span><ShieldCheck :size="14" />Protected by Release Station guardrails</span><span>v0.1.0 · Foundation milestone</span></footer>
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
      <div v-if="discoveryOpen" class="palette-backdrop" @click.self="closeDiscovery">
        <section ref="discoveryDialog" class="discovery-dialog" role="dialog" aria-modal="true" aria-labelledby="discovery-title">
          <header class="discovery-header">
            <div><div class="panel-kicker"><Globe2 :size="13" /> WEB STATION DISCOVERY</div><h2 id="discovery-title">Import hosted applications</h2><p>Read-only scan of configured Web Station roots. Nothing in Web Station is changed.</p></div>
            <button class="icon-button" type="button" aria-label="Close discovery" @click="closeDiscovery"><X :size="17" /></button>
          </header>
          <div class="discovery-phase"><span class="status-dot" :class="{ 'discovery-pulse': discoveryLoading }" />{{ discoveryPhase }}</div>
          <div v-if="discoveryError" class="discovery-error"><CircleAlert :size="16" />{{ discoveryError }}</div>
          <div v-if="importMessage" class="discovery-success" :class="{ 'is-closing': importClosing }"><Check :size="16" />{{ importMessage }}</div>
          <div v-if="importClosing" class="discovery-close-countdown" role="status" aria-live="polite"><span>Modalul se va închide în</span><strong :key="importCountdown">{{ importCountdown }}</strong><small>Poți continua după actualizarea listei.</small></div>
          <div v-if="!discoveryLoading && discoveredSites.length === 0 && !discoveryError" class="discovery-empty"><Globe2 :size="22" /><strong>No applications discovered</strong><span>Check that Web Station document roots exist under the configured read-only roots.</span></div>
          <div v-else class="discovery-list">
            <label v-for="site in discoveredSites" :key="site.project_root" class="discovery-row" :class="{ managed: site.already_managed }">
              <input v-model="selectedDiscoveredPaths" type="checkbox" :value="site.project_root" :disabled="site.already_managed">
              <span class="discovery-copy"><strong>{{ site.hostname || site.name }}</strong><small>{{ site.framework }} · {{ site.web_root }}</small><small>{{ site.permissions.message }}</small></span>
              <span class="discovery-badge" :class="site.permissions.status">{{ site.already_managed ? 'Managed' : site.permissions.status }}</span>
            </label>
          </div>
          <footer class="discovery-footer"><div class="discovery-selection"><span>{{ selectedDiscoveredPaths.length }} / {{ selectableDiscoveredPaths.length }} selected</span><button class="button button-secondary" type="button" :disabled="!selectableDiscoveredPaths.length || allDiscoverableSelected" @click="selectAllDiscovered">Select all</button><button class="button button-secondary" type="button" :disabled="!selectedDiscoveredPaths.length" @click="selectNoneDiscovered">Select none</button></div><div><button class="button button-secondary" type="button" @click="openDiscovery">Rescan</button><button class="button button-primary" type="button" :disabled="discoveryLoading || !selectedDiscoveredPaths.length" @click="importSelectedSites">Import selected</button></div></footer>
        </section>
      </div>
    </Transition>

    <Transition name="palette-fade">
      <div v-if="wizardOpen" class="palette-backdrop" @click.self="closeWizard">
        <section class="wizard-dialog" role="dialog" aria-modal="true" aria-labelledby="wizard-title">
          <header class="wizard-header">
            <div><div class="panel-kicker"><Plus :size="13" /> NEW SITE</div><h2 id="wizard-title">Add a site</h2><p>Configure the address, source repository and safe synchronization behavior.</p></div>
            <button class="icon-button" type="button" aria-label="Close wizard" @click="closeWizard"><X :size="17" /></button>
          </header>
          <div class="wizard-steps" aria-label="Wizard progress"><span v-for="step in 4" :key="step" class="wizard-step" :class="{ active: wizardStep === step, complete: wizardStep > step }"><b>{{ wizardStep > step ? '✓' : step }}</b><small>{{ ['Site', 'Repository', 'Sync', 'Review'][step - 1] }}</small></span></div>

          <div v-if="wizardStep === 1" class="wizard-body">
            <div class="wizard-intro"><Globe2 :size="20" /><div><strong>Where is this site?</strong><span>URL-ul este adresa publică. Căile sunt locațiile reale de pe NAS pe care Release Station le va verifica și actualiza.</span></div></div>
            <div class="form-grid"><label><span>Site name</span><input v-model="wizardForm.name" type="text" placeholder="My WordPress site" /></label><label><span>Public site URL</span><input v-model="wizardForm.url" type="url" placeholder="https://example.com" /></label></div>
            <label><span>Project root on Synology</span><input v-model="wizardForm.projectRoot" type="text" placeholder="/volume1/www/example.com" /><small>Directorul proiectului, nu URL-ul. Trebuie să existe deja pe NAS și să fie accesibil serviciului Release Station.</small></label>
            <label><span>Web/document root <em>optional</em></span><input v-model="wizardForm.webRoot" type="text" placeholder="Auto-detect from framework" /><small>Pentru Laravel, Symfony și Flarum se folosește automat <code>public/</code> dacă lași câmpul gol.</small></label>
            <label><span>Framework detection</span><select v-model="wizardForm.framework"><option value="auto">Auto-detect (recommended)</option><option value="wordpress">WordPress</option><option value="laravel">Laravel</option><option value="symfony">Symfony</option><option value="flarum">Flarum</option><option value="node">Node.js</option><option value="php">PHP</option><option value="unknown">Other / unknown</option></select></label>
          </div>

          <div v-else-if="wizardStep === 2" class="wizard-body">
            <div class="wizard-intro"><GitBranch :size="20" /><div><strong>Ce repository va alimenta site-ul?</strong><span>Alege un repository acordat GitHub App sau introdu manual un URL. Repository-urile private apar aici numai după instalarea App-ului cu acces selectat.</span></div></div>
            <div class="form-grid"><label><span>Provider</span><select v-model="wizardForm.provider"><option value="github">GitHub</option><option value="gitlab">GitLab</option><option value="bitbucket">Bitbucket</option><option value="generic">Other Git server</option></select></label><label><span>Branch</span><select v-if="wizardForm.provider === 'github'" v-model="wizardForm.branch" :disabled="!wizardForm.githubFullName || githubBranchesLoading"><option v-if="!wizardForm.githubFullName" value="">Select repository first</option><option v-else-if="githubBranchesLoading" value="">Loading branches…</option><option v-for="branch in githubBranches" :key="branch" :value="branch">{{ branch }}</option></select><input v-else v-model="wizardForm.branch" type="text" placeholder="main" /></label></div>
            <label v-if="wizardForm.provider === 'github'"><span>Repository from GitHub App</span><select :value="wizardForm.githubFullName" @change="onGithubRepositoryChange"><option value="">Select a granted repository or enter URL below</option><optgroup v-for="group in githubRepositoryGroups" :key="group.account" :label="`@${group.account}`"><option v-for="repository in group.repositories" :key="`${repository.installation_id}:${repository.id}`" :value="repository.full_name">{{ repository.name }}{{ repository.private ? ' · private' : '' }}</option></optgroup></select><small v-if="githubLoading">Loading repositories…</small><small v-else-if="githubState.connected && !githubRepositories.length">No granted repositories found. Add another GitHub account or refresh access.</small><button v-if="githubState.connected" class="text-button" type="button" @click="installGithubApp">Add another GitHub account <ArrowUpRight :size="13" /></button></label>
            <label><span>Repository clone URL</span><input v-model="wizardForm.cloneUrl" type="text" placeholder="https://github.com/matrixn/my-site.git" /></label>
            <div v-if="githubBranchesError" class="wizard-hint warning"><CircleAlert :size="15" /><span>{{ githubBranchesError }}</span></div><div v-if="githubState.connected" class="wizard-hint"><Check :size="15" /><span>GitHub App este conectată. Repository-urile private din listă sunt cele aprobate explicit în GitHub.</span></div><div v-else class="wizard-hint warning"><CircleAlert :size="15" /><span>GitHub App nu este conectată. Poți continua cu un URL public/manual sau poți instala App-ul din Settings.</span></div>
          </div>

          <div v-else-if="wizardStep === 3" class="wizard-body">
            <div class="wizard-intro"><RotateCw :size="20" /><div><strong>Cum se sincronizează?</strong><span>Alege comportamentul potrivit pentru infrastructura ta. Poți schimba strategia ulterior când activăm pipeline-ul complet.</span></div></div>
            <div class="strategy-grid"><label class="strategy-card" :class="{ selected: wizardForm.strategy === 'in_place' }"><input v-model="wizardForm.strategy" type="radio" value="in_place" /><span class="strategy-title">In-place <em>recommended</em></span><span>Actualizează directorul existent Web Station. Este potrivit pentru site-urile deja instalate și păstrează structura și datele locale.</span></label><label class="strategy-card" :class="{ selected: wizardForm.strategy === 'atomic' }"><input v-model="wizardForm.strategy" type="radio" value="atomic" /><span class="strategy-title">Atomic releases</span><span>Pregătește release-uri separate și activează unul doar după verificări. Oferă rollback mai sigur, dar necesită layout compatibil cu release-uri.</span></label></div>
            <div class="wizard-hint"><CircleHelp :size="15" /><span>Release Station nu execută încă sincronizarea la salvarea wizard-ului; acum înregistrează configurația și verifică rădăcinile. Deploy-ul va folosi această alegere.</span></div>
          </div>

          <div v-else class="wizard-body">
            <div class="wizard-intro"><Check :size="20" /><div><strong>Verifică configurația</strong><span>Site-ul va fi adăugat în catalog. Poți reveni ulterior în setările lui pentru ajustări.</span></div></div>
            <dl class="review-list"><div><dt>Site</dt><dd>{{ wizardForm.name }} · {{ wizardForm.url }}</dd></div><div><dt>Project root</dt><dd><code>{{ wizardForm.projectRoot }}</code></dd></div><div><dt>Repository</dt><dd>{{ wizardForm.provider }} · {{ wizardForm.cloneUrl }} · {{ wizardForm.branch }}</dd></div><div><dt>Framework / root</dt><dd>{{ wizardForm.framework }} · {{ wizardForm.webRoot || 'auto-detect' }}</dd></div><div><dt>Synchronization</dt><dd>{{ wizardForm.strategy === 'in_place' ? 'In-place' : 'Atomic releases' }}</dd></div></dl>
          </div>

          <div v-if="wizardError" class="discovery-error wizard-error"><CircleAlert :size="16" />{{ wizardError }}</div>
          <footer class="wizard-footer"><span>Step {{ wizardStep }} of 4</span><div><button class="button button-secondary" type="button" :disabled="wizardSaving" @click="wizardStep === 1 ? closeWizard() : previousWizardStep()">{{ wizardStep === 1 ? 'Cancel' : 'Back' }}</button><button v-if="wizardStep < 4" class="button button-primary" type="button" @click="nextWizardStep">Continue <ArrowUpRight :size="14" /></button><button v-else class="button button-primary" type="button" :disabled="wizardSaving" @click="createManualSite">{{ wizardSaving ? 'Saving…' : 'Create site' }} <Check :size="14" /></button></div></footer>
        </section>
      </div>
    </Transition>
  </div>
</template>
