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
  runtime?: Record<string, any>;
  created_at: string;
  updated_at: string;
  repository?: { provider: string; clone_url: string; branch: string };
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
type Deployment = { id: string; site_id: string; trigger_type: string; deployment_method?: string; branch: string; commit_sha: string; commit_message: string; commit_url: string; status: string; error_code?: string; error_summary?: string; queued_at: string; started_at?: string; finished_at?: string; duration_ms?: number; created_at: string; build_log?: string; deployment_log?: string; };
type Commit = { sha: string; message: string; branch: string; author?: string; url?: string; created_at?: string; deployed: boolean; deployment_id?: string; status: string; };

type GithubState = {
  configured: boolean;
  mode: 'managed' | 'self_hosted' | string;
  configuration_error: string;
  connected: boolean;
  app_slug: string;
  setup_url: string;
  account_login?: string;
  installations: GithubInstallation[];
};

const healthState = ref<HealthState>('checking');
const isDark = ref(true);
const commandOpen = ref(false);
const commandQuery = ref('');
const activeNav = ref('Dashboard');
const selectedSite = ref<Site | null>(null);
const selectedSiteTab = ref<'Overview' | 'Deployments'>('Overview');
const selectedDeployment = ref<Deployment | null>(null);
const siteDeployments = ref<Deployment[]>([]);
const siteCommits = ref<Commit[]>([]);
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
const githubState = ref<GithubState>({ configured: false, mode: 'managed', configuration_error: '', connected: false, app_slug: '', setup_url: '', installations: [] });
const githubRepositories = ref<GithubRepository[]>([]);
const githubLoading = ref(false);
const githubAccount = ref('');
const githubSaving = ref(false);
const githubMessage = ref('');
const githubError = ref('');
const githubConnectState = ref<'idle' | 'starting' | 'waiting'>('idle');
const deployingSiteId = ref('');
const deployMessage = ref('');
const deployError = ref('');
let githubPollTimer: ReturnType<typeof setInterval> | undefined;
let importCloseTimer: ReturnType<typeof setTimeout> | undefined;
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
  return { configured: false, mode: 'managed', configuration_error: 'Zion Connector is not provisioned for this ReleaseStation instance', connected: false, app_slug: '', setup_url: '', installations: [] };
}

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

async function loadSiteHistory() {
  if (!selectedSite.value) return;
  siteDetailLoading.value = true;
  try {
    const [deploymentResponse, commitResponse] = await Promise.all([
      fetch(`/releasestation/api/v1/sites/${selectedSite.value.id}/deployments?page=${sitePage.value}&per_page=25&q=${encodeURIComponent(deploymentSearch.value)}`, { headers: { Accept: 'application/json' } }),
      fetch(`/releasestation/api/v1/sites/${selectedSite.value.id}/commits`, { headers: { Accept: 'application/json' } }),
    ]);
    const deploymentPayload = await deploymentResponse.json().catch(() => ({}));
    const commitPayload = await commitResponse.json().catch(() => ({}));
    if (!deploymentResponse.ok) throw new Error(deploymentPayload.error?.message || 'Could not load deployments.');
    siteDeployments.value = deploymentPayload.data?.items || [];
    siteTotalPages.value = deploymentPayload.data?.total_pages || 1;
    siteCommits.value = commitResponse.ok ? (commitPayload.data || []) : [];
  } catch (error) {
    deployError.value = error instanceof Error ? error.message : 'Could not load site history.';
  } finally {
    siteDetailLoading.value = false;
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
  } catch (error) {
    deployError.value = error instanceof Error ? error.message : 'Could not load deployment details.';
  } finally {
    deploymentDetailLoading.value = false;
  }
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
    deployMessage.value = `${commit.sha.slice(0, 7)} deployed successfully.`;
    await loadSiteHistory();
  } catch (error) {
    deployError.value = error instanceof Error ? error.message : 'Deployment failed.';
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
    return;
  }
  wizardForm.value.provider = 'github';
  wizardForm.value.githubInstallationId = repository.installation_id;
  wizardForm.value.githubRepositoryId = repository.id;
  wizardForm.value.githubFullName = repository.full_name;
  wizardForm.value.githubDefaultBranch = repository.default_branch;
  wizardForm.value.cloneUrl = repository.clone_url;
  wizardForm.value.branch = repository.default_branch;
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
    activeNav.value = 'Sites';
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
    deployMessage.value = `${site.hostname || site.name} deployed atomically.`;
    await loadSites();
  } catch (error) {
    deployError.value = error instanceof Error ? error.message : 'Deployment failed.';
  } finally {
    deployingSiteId.value = '';
  }
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
    window.requestAnimationFrame(() => discoveryDialog.value?.scrollTo({ top: 0, behavior: 'smooth' }));
    startImportCloseCountdown();
  } catch {
    clearImportCloseTimer();
    importClosing.value = false;
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
  loadGithubStatus();
});

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown);
  if (githubPollTimer) clearInterval(githubPollTimer);
  clearImportCloseTimer();
});
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
        <button class="nav-item" :class="{ active: activeNav === 'Settings' }" type="button" @click="activeNav = 'Settings'"><Settings2 :size="17" /><span>Settings</span></button>
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

        <section v-if="activeNav === 'Dashboard' && githubState.mode === 'managed'" class="connection-grid">
          <article class="panel github-panel">
            <div class="panel-heading"><div><div class="panel-kicker"><GitBranch :size="13" /> SOURCE CONTROL</div><h2>GitHub connection</h2></div><span class="connection-badge" :class="{ connected: githubState.connected }"><span class="status-dot" />{{ githubState.connected ? 'Connected' : 'Not connected' }}</span></div>
            <p v-if="githubState.connected" class="connection-copy">GitHub este conectat prin connectorul Zion și poate accesa repository-urile private permise în GitHub{{ githubState.account_login ? ` pentru ${githubState.account_login}` : '' }}.</p>
            <p v-else class="connection-copy">Conectează GitHub în câteva secunde. Vei fi trimis la GitHub să te autentifici și să instalezi aplicația Zion în contul sau organizația ta.</p>
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

        <div v-if="deployMessage || deployError" class="deployment-toast" :class="{ error: deployError }"><Check v-if="deployMessage" :size="15" />{{ deployMessage || deployError }}</div>

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
          <div class="site-tabs"><button type="button" :class="{ active: selectedSiteTab === 'Overview' }" @click="selectedSiteTab = 'Overview'">Overview</button><button type="button" :class="{ active: selectedSiteTab === 'Deployments' }" @click="selectedSiteTab = 'Deployments'">Deployments <span>{{ siteDeployments.length }}</span></button></div>
          <template v-if="selectedSiteTab === 'Overview'">
            <div class="site-meta-grid"><article class="panel site-meta-card"><span>Framework</span><strong>{{ frameworkLabel(selectedSite.framework) }}</strong></article><article class="panel site-meta-card"><span>PHP</span><strong>{{ siteRuntime(selectedSite, 'php_version', '8.5') }}</strong></article><article class="panel site-meta-card"><span>HTTP server</span><strong>{{ siteRuntime(selectedSite, 'http_server', 'Detected by Web Station') }}</strong></article><article class="panel site-meta-card"><span>Public IP</span><button type="button" @click="copyPublicIP(selectedSite)">{{ siteRuntime(selectedSite, 'public_ip', 'Not detected') }}</button></article><article class="panel site-meta-card"><span>Public Web</span><a :href="sitePublicURL(selectedSite)" target="_blank" rel="noreferrer">{{ sitePublicURL(selectedSite) }}</a></article><article class="panel site-meta-card"><span>Created</span><strong>{{ new Date(selectedSite.created_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }) }}</strong></article></div>
            <div class="detail-section-heading"><div><div class="panel-kicker">SOURCE HISTORY</div><h2>Commits</h2></div><span class="muted-copy">{{ selectedSite.repository?.branch || 'main' }}</span></div>
            <div v-if="siteDetailLoading" class="detail-loading"><RotateCw :size="16" class="spin" /> Loading repository history…</div>
            <div v-else-if="siteCommits.length" class="commit-table"><div v-for="commit in siteCommits" :key="commit.sha" class="commit-row" :class="commit.status"><span class="commit-state" :class="commit.status" v-if="commit.deployed"><Check :size="13" /></span><span class="commit-state failed" v-else-if="commit.status === 'failed'"><CircleX :size="13" /></span><span class="commit-state pending" v-else><CircleDot :size="13" /></span><code>{{ commit.sha.slice(0, 7) }}</code><span class="commit-message"><strong>{{ commit.message.split('\n')[0] }}</strong><small>{{ commit.author || 'GitHub' }} · {{ relativeTime(commit.created_at) }}</small></span><span class="commit-status">{{ commit.deployed ? 'Deployed' : (commit.status === 'failed' ? 'Failed' : 'Not deployed') }}</span><button class="button button-secondary commit-deploy" type="button" :disabled="deployingCommitSha !== '' || commit.deployed || selectedSite.strategy !== 'atomic'" @click="deployCommit(commit)"><RotateCw v-if="deployingCommitSha === commit.sha" :size="13" class="spin" /><span v-else>Deploy</span></button></div></div>
            <div v-else class="management-empty"><GitBranch :size="22" /><strong>No commits available</strong><span>Connect GitHub and configure a repository for this site to see commit history.</span></div>
          </template>
          <template v-else>
            <div class="deployments-toolbar"><div><div class="panel-kicker">DEPLOYMENT HISTORY</div><h2>Deployments</h2></div><input v-model="deploymentSearch" type="search" placeholder="Search commit name…" @change="sitePage = 1; loadSiteHistory()" /></div>
            <div v-if="siteDeployments.length" class="deployment-history-list"><button v-for="deployment in siteDeployments" :key="deployment.id" class="deployment-history-row" type="button" @click="openDeploymentDetails(deployment)"><span class="commit-state" :class="deployment.status"><Check v-if="deployment.status === 'deployed'" :size="13" /><RotateCw v-else-if="deployment.status === 'running'" :size="13" class="spin" /><CircleX v-else :size="13" /></span><code>{{ deployment.commit_sha ? deployment.commit_sha.slice(0, 7) : 'unknown' }}</code><span class="commit-message"><strong>{{ deployment.commit_message || 'Deployment without commit metadata' }}</strong><small>{{ deployment.branch || '—' }} · {{ relativeTime(deployment.created_at) }} · {{ durationLabel(deployment.duration_ms) }}</small></span><span class="discovery-badge" :class="deployment.status">{{ deployment.status }}</span><span class="row-arrow">→</span></button></div><div v-else class="management-empty"><Clock3 :size="22" /><strong>No deployments yet</strong><span>Deploy a commit from Overview to build the deployment history.</span></div>
            <div class="pagination"><button type="button" :disabled="sitePage <= 1" @click="setDeploymentPage(sitePage - 1)">Previous</button><span>Page {{ sitePage }} of {{ siteTotalPages }}</span><button type="button" :disabled="sitePage >= siteTotalPages" @click="setDeploymentPage(sitePage + 1)">Next</button></div>
          </template>
        </section>

        <section v-if="activeNav === 'DeploymentDetail' && selectedSite && selectedDeployment" class="deployment-detail-view">
          <div class="deployment-detail-header"><div><button class="text-button" type="button" @click="backToSiteTab('Deployments')">← Back to deployments</button><h1>Deployment details <span>·</span> <code>{{ selectedDeployment.commit_sha ? selectedDeployment.commit_sha.slice(0, 7) : 'unknown' }}</code></h1><div class="deployment-detail-meta"><span class="commit-state" :class="selectedDeployment.status"><Check v-if="selectedDeployment.status === 'deployed'" :size="13" /><CircleX v-else :size="13" /></span><strong>{{ selectedDeployment.status }}</strong><span>{{ selectedDeployment.branch }}</span><span>·</span><span>{{ selectedDeployment.commit_message || 'No commit message' }}</span><span>·</span><span>{{ relativeTime(selectedDeployment.finished_at || selectedDeployment.created_at) }}</span><span>·</span><span>{{ durationLabel(selectedDeployment.duration_ms) }}</span></div></div><div class="hero-actions"><a v-if="sitePublicURL(selectedSite).startsWith('http')" class="button button-secondary" :href="sitePublicURL(selectedSite)" target="_blank" rel="noreferrer"><Globe2 :size="15" />Visit</a><button class="button button-secondary" type="button" @click="logsExpanded = !logsExpanded">{{ logsExpanded ? 'Collapse all' : 'Expand all' }}</button></div></div>
          <div class="detail-facts"><span><strong>Method</strong>{{ selectedDeployment.deployment_method || selectedDeployment.trigger_type || 'manual' }}</span><span><strong>Branch</strong>{{ selectedDeployment.branch || '—' }}</span><span><strong>Commit</strong>{{ selectedDeployment.commit_sha || '—' }}</span><span><strong>Started</strong>{{ selectedDeployment.started_at || '—' }}</span></div>
          <details :open="logsExpanded" class="log-panel"><summary><span class="commit-state deployed"><Check :size="13" /></span><strong>Build logs</strong><span>{{ durationLabel(selectedDeployment.duration_ms) }}</span></summary><pre>{{ selectedDeployment.build_log || 'No build output was captured.' }}</pre></details>
          <details :open="logsExpanded" class="log-panel"><summary><span class="commit-state deployed"><Check :size="13" /></span><strong>Deployment logs</strong><span>{{ durationLabel(selectedDeployment.duration_ms) }}</span></summary><pre>{{ selectedDeployment.deployment_log || 'No deployment output was captured.' }}</pre></details>
        </section>

        <section v-if="activeNav === 'Settings'" class="settings-view">
          <div class="sites-management-header"><div><div class="eyebrow"><span class="eyebrow-pulse" /> WORKSPACE SETTINGS</div><h1>Settings.</h1><p class="hero-copy">Connect the services ReleaseStation uses to discover repositories and deploy sites.</p></div></div>
          <article v-if="githubState.mode === 'managed'" class="panel settings-card">
            <div class="settings-card-heading"><span class="settings-icon"><GitBranch :size="18" /></span><div><div class="panel-kicker">SOURCE CONTROL CONNECTOR</div><h2>GitHub</h2><p>Conectează GitHub fără să creezi o aplicație proprie și fără să încarci cheia PEM pe NAS.</p></div><span class="connection-badge" :class="{ connected: githubState.connected }"><span class="status-dot" />{{ githubState.connected ? 'Connected' : 'Not connected' }}</span></div>
            <div class="settings-form"><div class="connector-status"><span class="status-dot" :class="{ 'status-dot-warning': !githubState.connected }" /><strong>{{ githubState.connected ? 'GitHub connected through Zion' : 'Connect GitHub to continue' }}</strong><small v-if="githubState.account_login">Account: {{ githubState.account_login }}</small><small v-else-if="githubState.configuration_error">{{ githubState.configuration_error }}</small></div><p class="settings-explanation">Apasă butonul, autentifică-te pe GitHub și instalează aplicația Zion în contul sau organizația ta. Selectezi exact repository-urile private accesibile. Cheia GitHub App rămâne în serviciul Zion, nu pe NAS.</p></div>
            <div v-if="githubError" class="discovery-error"><CircleAlert :size="16" />{{ githubError }}</div><div v-if="githubMessage" class="discovery-success"><Check :size="16" />{{ githubMessage }}</div>
            <div class="settings-actions"><span /><button class="button button-secondary" type="button" @click="loadGithubStatus">Refresh status</button><button class="button button-primary" type="button" @click="installGithubApp" :disabled="githubConnectState !== 'idle'">{{ githubConnectState === 'waiting' ? 'Waiting for GitHub…' : (githubState.connected ? 'Manage GitHub' : 'Connect GitHub') }} <ArrowUpRight :size="14" /></button></div>
            <div class="settings-note"><CircleHelp :size="15" /><span>Acesta este modul recomandat pentru clienții ReleaseStation: o singură Zion GitHub App, instalată separat în contul fiecărui client. Nu este nevoie de App ID, slug sau fișier PEM pe NAS.</span></div>
            <div v-if="githubState.installations.length" class="installation-list"><div v-for="installation in githubState.installations" :key="installation.github_installation_id" class="installation-row"><GitBranch :size="15" /><span><strong>{{ installation.account_login }}</strong><small>{{ installation.account_type }} · {{ installation.repository_selection }} repositories · installation {{ installation.github_installation_id }}</small></span><button class="button button-secondary" type="button" @click="loadGithubRepositories">Refresh repositories</button></div></div>
          </article>
          <article v-if="githubState.mode !== 'managed'" class="panel settings-card">
            <div class="settings-card-heading"><span class="settings-icon"><GitBranch :size="18" /></span><div><div class="panel-kicker">SOURCE CONTROL CONNECTOR</div><h2>GitHub App</h2><p>Instalează App-ul GitHub și selectează explicit unul sau mai multe repository-uri, inclusiv private.</p></div><span class="connection-badge" :class="{ connected: githubState.connected }"><span class="status-dot" />{{ githubState.connected ? 'Connected' : 'Not connected' }}</span></div>
            <div class="settings-form"><div class="connector-status"><span class="status-dot" :class="{ 'status-dot-warning': !githubState.configured }" /><strong>{{ githubState.configured ? 'App credentials detected on NAS' : 'App credentials are not configured' }}</strong><small v-if="githubState.configuration_error">{{ githubState.configuration_error }}</small><small v-else>App: {{ githubState.app_slug }}</small></div><p class="settings-explanation">Private repository access is granted in GitHub during installation. ReleaseStation receives only short-lived installation tokens and never stores a PAT.</p></div>
            <div v-if="githubError" class="discovery-error"><CircleAlert :size="16" />{{ githubError }}</div><div v-if="githubMessage" class="discovery-success"><Check :size="16" />{{ githubMessage }}</div>
            <div class="settings-actions"><span /><button class="button button-secondary" type="button" @click="loadGithubStatus">Refresh status</button><button v-if="githubState.configured" class="button button-primary" type="button" @click="installGithubApp">Install / manage GitHub App <ArrowUpRight :size="14" /></button></div>
            <div class="settings-note"><CircleHelp :size="15" /><span>Setează Setup URL-ul GitHub App la <code>{{ githubState.setup_url || '/releasestation/api/v1/integrations/github/setup' }}</code>, apoi folosește butonul de mai sus. După instalare, repository-urile private selectate în GitHub apar în wizard.</span></div>
            <div v-if="githubState.installations.length" class="installation-list"><div v-for="installation in githubState.installations" :key="installation.github_installation_id" class="installation-row"><GitBranch :size="15" /><span><strong>{{ installation.account_login }}</strong><small>{{ installation.account_type }} · {{ installation.repository_selection }} repositories · installation {{ installation.github_installation_id }}</small></span><button class="button button-secondary" type="button" @click="loadGithubRepositories">Refresh repositories</button></div></div>
          </article>
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
          <footer class="discovery-footer"><span>{{ selectedDiscoveredPaths.length }} selected</span><div><button class="button button-secondary" type="button" @click="openDiscovery">Rescan</button><button class="button button-primary" type="button" :disabled="discoveryLoading || !selectedDiscoveredPaths.length" @click="importSelectedSites">Import selected</button></div></footer>
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
            <div class="wizard-intro"><Globe2 :size="20" /><div><strong>Where is this site?</strong><span>URL-ul este adresa publică. Căile sunt locațiile reale de pe NAS pe care ReleaseStation le va verifica și actualiza.</span></div></div>
            <div class="form-grid"><label><span>Site name</span><input v-model="wizardForm.name" type="text" placeholder="My WordPress site" /></label><label><span>Public site URL</span><input v-model="wizardForm.url" type="url" placeholder="https://example.com" /></label></div>
            <label><span>Project root on Synology</span><input v-model="wizardForm.projectRoot" type="text" placeholder="/volume1/www/example.com" /><small>Directorul proiectului, nu URL-ul. Trebuie să existe deja pe NAS și să fie accesibil serviciului ReleaseStation.</small></label>
            <label><span>Web/document root <em>optional</em></span><input v-model="wizardForm.webRoot" type="text" placeholder="Auto-detect from framework" /><small>Pentru Laravel, Symfony și Flarum se folosește automat <code>public/</code> dacă lași câmpul gol.</small></label>
            <label><span>Framework detection</span><select v-model="wizardForm.framework"><option value="auto">Auto-detect (recommended)</option><option value="wordpress">WordPress</option><option value="laravel">Laravel</option><option value="symfony">Symfony</option><option value="flarum">Flarum</option><option value="node">Node.js</option><option value="php">PHP</option><option value="unknown">Other / unknown</option></select></label>
          </div>

          <div v-else-if="wizardStep === 2" class="wizard-body">
            <div class="wizard-intro"><GitBranch :size="20" /><div><strong>Ce repository va alimenta site-ul?</strong><span>Alege un repository acordat GitHub App sau introdu manual un URL. Repository-urile private apar aici numai după instalarea App-ului cu acces selectat.</span></div></div>
            <div class="form-grid"><label><span>Provider</span><select v-model="wizardForm.provider"><option value="github">GitHub</option><option value="gitlab">GitLab</option><option value="bitbucket">Bitbucket</option><option value="generic">Other Git server</option></select></label><label><span>Branch</span><input v-model="wizardForm.branch" type="text" placeholder="main" /></label></div>
            <label v-if="wizardForm.provider === 'github'"><span>Repository from GitHub App</span><select :value="wizardForm.githubFullName" @change="onGithubRepositoryChange"><option value="">Select a granted repository or enter URL below</option><option v-for="repository in githubRepositories" :key="`${repository.installation_id}:${repository.id}`" :value="repository.full_name">{{ repository.full_name }}{{ repository.private ? ' · private' : '' }} · {{ repository.account_login }}</option></select><small v-if="githubLoading">Loading repositories…</small><small v-else-if="githubState.connected && !githubRepositories.length">No granted repositories found. Refresh GitHub App access.</small></label>
            <label><span>Repository clone URL</span><input v-model="wizardForm.cloneUrl" type="text" placeholder="https://github.com/matrixn/my-site.git" /></label>
            <div v-if="githubState.connected" class="wizard-hint"><Check :size="15" /><span>GitHub App este conectată. Repository-urile private din listă sunt cele aprobate explicit în GitHub.</span></div><div v-else class="wizard-hint warning"><CircleAlert :size="15" /><span>GitHub App nu este conectată. Poți continua cu un URL public/manual sau poți instala App-ul din Settings.</span></div>
          </div>

          <div v-else-if="wizardStep === 3" class="wizard-body">
            <div class="wizard-intro"><RotateCw :size="20" /><div><strong>Cum se sincronizează?</strong><span>Alege comportamentul potrivit pentru infrastructura ta. Poți schimba strategia ulterior când activăm pipeline-ul complet.</span></div></div>
            <div class="strategy-grid"><label class="strategy-card" :class="{ selected: wizardForm.strategy === 'in_place' }"><input v-model="wizardForm.strategy" type="radio" value="in_place" /><span class="strategy-title">In-place <em>recommended</em></span><span>Actualizează directorul existent Web Station. Este potrivit pentru site-urile deja instalate și păstrează structura și datele locale.</span></label><label class="strategy-card" :class="{ selected: wizardForm.strategy === 'atomic' }"><input v-model="wizardForm.strategy" type="radio" value="atomic" /><span class="strategy-title">Atomic releases</span><span>Pregătește release-uri separate și activează unul doar după verificări. Oferă rollback mai sigur, dar necesită layout compatibil cu release-uri.</span></label></div>
            <div class="wizard-hint"><CircleHelp :size="15" /><span>ReleaseStation nu execută încă sincronizarea la salvarea wizard-ului; acum înregistrează configurația și verifică rădăcinile. Deploy-ul va folosi această alegere.</span></div>
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
