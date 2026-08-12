(function (global) {
  'use strict';

  if (!global.SYNO || !global.Vue) {
    return;
  }

  SYNO.namespace('SYNO.ZionReleaseStation');

  var styleId = 'zion-releasestation-native-style';
  var styles = [
    '.zion-native{height:100%;box-sizing:border-box;background:#f1f3f6;color:#20242a;font:13px -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}',
    '.zion-native *{box-sizing:border-box}',
    '.zion-native-header{display:flex;align-items:center;gap:12px;height:74px;padding:14px 20px;background:#fff;border-bottom:1px solid #d9e0e7}',
    '.zion-native-icon{width:42px;height:42px;border-radius:9px;box-shadow:0 4px 10px #203b5a24}',
    '.zion-native-title{flex:1}.zion-native-title strong{display:block;font-size:16px;font-weight:600}.zion-native-title span{display:block;margin-top:3px;color:#77828d;font-size:11px}',
    '.zion-native-nav{display:flex;gap:4px;padding:12px 20px;background:#fff;border-bottom:1px solid #e2e6eb}',
    '.zion-native-nav button{padding:7px 12px;border:0;border-radius:5px;color:#687482;background:transparent;font:inherit;font-size:12px;cursor:pointer}',
    '.zion-native-nav button.active{color:#176fbe;background:#e8f2fc;font-weight:600}',
    '.zion-native-body{height:calc(100% - 119px);overflow:auto;padding:22px 24px}',
    '.zion-native-intro{margin-bottom:18px}.zion-native-intro h1{margin:0 0 6px;font-size:21px;font-weight:600}.zion-native-intro p{margin:0;color:#687482;line-height:1.5}',
    '.zion-native-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}',
    '.zion-native-grid .github-connection-card{grid-column:1}',
    '.zion-native-card{padding:17px;background:#fff;border:1px solid #d9e0e7;border-radius:7px;box-shadow:0 2px 5px #1e33420b}',
    '.zion-native-card h2{margin:0 0 14px;font-size:14px;font-weight:600}.zion-native-card p{margin:0;color:#687482;font-size:12px;line-height:1.55}',
    '.zion-native-status{display:flex;align-items:center;gap:9px;margin-bottom:13px;font-weight:600}.zion-native-dot{width:9px;height:9px;border-radius:50%;background:#e0a326;box-shadow:0 0 0 4px #e0a3261c}.zion-native-dot.healthy{background:#1aa36f;box-shadow:0 0 0 4px #1aa36f1c}.zion-native-dot.offline{background:#d64b4b;box-shadow:0 0 0 4px #d64b4b1c}',
    '.zion-native-checks{display:grid;gap:8px;margin:0 0 13px;padding:0;list-style:none}.zion-native-check{display:flex;align-items:flex-start;gap:8px;color:#687482;font-size:12px;line-height:1.4}.zion-native-check b{width:17px;height:17px;display:grid;place-items:center;flex:0 0 auto;border-radius:50%;color:#fff;background:#b9c1ca;font-size:10px}.zion-native-check.ok{color:#35404c}.zion-native-check.ok b{background:#1aa36f}.zion-native-check.warn b{background:#e0a326}',
    '.zion-native-list{margin:0}.zion-native-list div{display:flex;justify-content:space-between;gap:20px;padding:8px 0;border-top:1px solid #edf0f3}.zion-native-list dt{color:#77828d}.zion-native-list dd{margin:0;text-align:right;font-weight:600}',
    '.zion-native-toggle{display:flex;align-items:flex-start;gap:10px;margin:2px 0 8px;cursor:pointer}.zion-native-toggle input{width:16px;height:16px;margin:1px 0 0;accent-color:#1677d2}.zion-native-toggle strong,.zion-native-toggle small{display:block}.zion-native-toggle strong{font-size:12px}.zion-native-toggle small{margin-top:3px;color:#77828d;font-size:11px;line-height:1.4}.zion-native-setting-state{margin:0 0 15px 26px;color:#77828d;font-size:11px}',
    '.zion-native-form{display:grid;gap:12px}.zion-native-form label{display:grid;gap:5px;color:#35404c;font-size:12px;font-weight:600}.zion-native-form input{width:100%;padding:8px 10px;border:1px solid #cbd2da;border-radius:5px;color:#20242a;background:#fff;font:inherit;font-size:12px}.zion-native-form small{color:#77828d;font-size:11px;font-weight:400;line-height:1.4}.zion-native-file{padding:7px!important;background:#f8fafc!important}.zion-native-form-message{min-height:17px;color:#687482;font-size:11px}.zion-native-form-message.error{color:#c23e3e}.zion-native-form-message.ok{color:#16845b}',
    '.zion-native-actions{display:flex;gap:8px;margin-top:17px}.zion-native-button{padding:8px 13px;border:1px solid #cbd2da;border-radius:5px;color:#35404c;background:#fff;font:inherit;font-size:12px;font-weight:600;cursor:pointer}.zion-native-button.primary{border-color:#1677d2;color:#fff;background:#1677d2}.zion-native-button:hover{filter:brightness(.97)}',
    '.zion-native-note{padding:12px 14px;border-left:3px solid #1677d2;background:#edf6ff;color:#53606d;font-size:12px;line-height:1.5}',
    '@media(max-width:700px){.zion-native-grid{grid-template-columns:1fr}.zion-native-body{padding:18px}}'
  ].join('');

  function installStyles() {
    if (document.getElementById(styleId)) return;
    var style = document.createElement('style');
    style.id = styleId;
    style.textContent = styles;
    document.head.appendChild(style);
  }

  function workspaceUrl() {
    return window.location.origin + '/releasestation/';
  }

  var template = [
    '<v-app-instance class-name="SYNO.ZionReleaseStation.Instance">',
    '  <v-app-window width="980" height="650" ref="appWindow" :resizable="true" syno-id="SYNO.ZionReleaseStation.Window">',
    '    <div class="zion-native">',
    '      <header class="zion-native-header">',
    '        <img class="zion-native-icon" src="/webman/3rdparty/zion-releasestation/images/app_64.png" alt="">',
    '        <div class="zion-native-title"><strong>Zion ReleaseStation</strong><span>Synology-native configuration</span></div>',
    '        <button v-if="webAccessEnabled" class="zion-native-button primary" type="button" @click="openWorkspace">Open workspace ↗</button>',
    '      </header>',
    '      <nav class="zion-native-nav" aria-label="ReleaseStation sections">',
    '        <button type="button" :class="{active: tab === \'overview\'}" @click="tab = \'overview\'">Overview</button>',
    '        <button type="button" :class="{active: tab === \'activation\'}" @click="tab = \'activation\'">Activation</button>',
    '        <button type="button" :class="{active: tab === \'settings\'}" @click="tab = \'settings\'">Configuration</button>',
    '      </nav>',
    '      <section class="zion-native-body">',
    '        <div v-if="tab === \'overview\'">',
    '          <div class="zion-native-intro"><h1>ReleaseStation control plane</h1><p>Manage activation and package configuration here. The full deployment workspace remains available through the web fallback.</p></div>',
    '          <div class="zion-native-grid">',
    '            <article class="zion-native-card"><h2>Package health</h2><div class="zion-native-status"><span class="zion-native-dot" :class="healthClass"></span>{{ healthTitle }}</div><p>{{ healthDetail }}</p><div class="zion-native-actions"><button class="zion-native-button" type="button" @click="checkHealth">Refresh</button></div></article>',
    '            <article class="zion-native-card github-connection-card"><h2>GitHub App connection</h2><ul class="zion-native-checks"><li v-for="check in githubChecks" :key="check.label" class="zion-native-check" :class="check.ok ? \'ok\' : \'warn\'"><b>{{ check.ok ? \'✓\' : \'!\' }}</b><span>{{ check.label }}</span></li></ul><p>{{ githubSummary }}</p><div class="zion-native-actions"><button class="zion-native-button" type="button" @click="loadGithub">Refresh</button><button class="zion-native-button primary" type="button" @click="tab = \'settings\'">Configure</button></div></article>',
    '            <article class="zion-native-card"><h2>Runtime</h2><dl class="zion-native-list"><div><dt>Version</dt><dd>{{ health.version || \'—\' }}</dd></div><div><dt>Platform</dt><dd>{{ health.platform || \'—\' }}</dd></div><div><dt>Database</dt><dd>{{ health.database || \'—\' }}</dd></div><div><dt>API</dt><dd>127.0.0.1:24871</dd></div></dl></article>',
    '          </div>',
    '        </div>',
    '        <div v-else-if="tab === \'activation\'">',
    '          <div class="zion-native-intro"><h1>Activation</h1><p>License activation is managed from the native DSM surface and never requires exposing the backend daemon publicly.</p></div>',
    '          <article class="zion-native-card"><div class="zion-native-status"><span class="zion-native-dot healthy"></span>Foundation edition ready</div><p>The activation workflow is prepared for the ReleaseStation licensing service. Until a license is connected, the local package remains available for foundation development.</p><div class="zion-native-actions"><button class="zion-native-button primary" type="button" disabled>Connect license</button></div></article>',
    '        </div>',
    '        <div v-else>',
    '          <div class="zion-native-intro"><h1>Package configuration</h1><p>These values are provided by the DSM package and are intentionally bound to the local service.</p></div>',
    '          <article class="zion-native-card">',
    '            <h2>Web workspace</h2>',
    '            <label class="zion-native-toggle"><input type="checkbox" v-model="webAccessEnabled" :disabled="webAccessState === \'loading\'" @change="saveWebAccess"><span><strong>Enable /releasestation/ URL</strong><small>Expose the web fallback through the DSM HTTPS route.</small></span></label>',
    '            <div class="zion-native-setting-state" role="status">{{ webAccessMessage }}</div>',
    '            <div v-if="webAccessEnabled">',
    '              <dl class="zion-native-list"><div><dt>Service bind address</dt><dd>127.0.0.1</dd></div><div><dt>Service port</dt><dd>24871</dd></div><div><dt>DSM route</dt><dd>/releasestation/</dd></div><div><dt>Data store</dt><dd>SQLite</dd></div></dl>',
    '              <div class="zion-native-note">The web workspace is available as a fallback and advanced management surface. It remains protected by the DSM HTTPS entry point; the local service is not exposed directly.</div>',
    '            </div>',
    '            <div v-else class="zion-native-note">The /releasestation/ web route is disabled. Related URL and service settings are hidden.</div>',
    '          </article>',
    '          <article v-if="github.mode === \'managed\'" class="zion-native-card zion-native-github-settings">',
    '            <h2>Connect GitHub</h2>',
    '            <p>Conectează GitHub prin aplicația Zion fără să creezi o GitHub App proprie și fără să încarci o cheie PEM pe NAS.</p>',
    '            <div class="zion-native-status"><span class="zion-native-dot" :class="github.connected ? \'healthy\' : \'warning\'"></span>{{ github.connected ? \'GitHub connected through Zion\' : \'GitHub is not connected\' }}</div>',
    '            <p v-if="github.account_login">Account: {{ github.account_login }}</p>',
    '            <div class="zion-native-actions"><button class="zion-native-button primary" type="button" @click="installGithubApp">{{ github.connected ? \'Manage GitHub\' : \'Connect GitHub\' }} ↗</button><button class="zion-native-button" type="button" @click="loadGithub">Refresh</button></div>',
    '            <div class="zion-native-note">Pașii se deschid în GitHub: autentificare, alegerea contului/organizației și selectarea repository-urilor private. Cheia aplicației rămâne în serviciul Zion.</div>',
    '          </article>',
    '          <article v-if="github.mode !== \'managed\'" class="zion-native-card zion-native-github-settings">',
    '            <h2>GitHub App connector</h2>',
    '            <p>Configurează aici App-ul GitHub pentru acces la repository-uri private. Nu introduci PAT; ReleaseStation folosește tokenuri temporare de instalare.</p>',
    '            <form class="zion-native-form" @submit.prevent="saveGithubConfig">',
    '              <label>GitHub App ID<input v-model.trim="githubConfig.app_id" type="text" placeholder="123456" autocomplete="off"><small>ID-ul numeric din pagina GitHub App.</small></label>',
    '              <label>GitHub App slug<input v-model.trim="githubConfig.app_slug" type="text" placeholder="zion-releasestation" autocomplete="off"><small>Slug-ul folosit în URL-ul de instalare.</small></label>',
    '              <label>Setup URL<input v-model.trim="githubConfig.setup_url" type="url" placeholder="https://raduta.synology.me:5001/releasestation/api/v1/integrations/github/setup"><small>URL public HTTPS configurat și în GitHub App. Trebuie să ajungă la NAS prin reverse proxy/DSM.</small></label>',
    '              <div class="zion-native-actions"><button class="zion-native-button primary" type="submit" :disabled="githubConfigState === \'saving\'">Save App settings</button></div>',
    '            </form>',
    '            <div class="zion-native-actions"><input class="zion-native-file" type="file" accept=".pem,.key,application/x-pem-file" @change="uploadGithubKey"><button class="zion-native-button primary" type="button" @click="installGithubApp" :disabled="!github.configured">Install / manage in GitHub</button></div>',
    '            <div class="zion-native-form-message" :class="githubMessageClass" role="status">{{ githubMessage }}</div>',
    '            <div class="zion-native-note"><strong>Pași:</strong> creează GitHub App cu Contents/Metadata Read-only; copiază App ID și slug; configurează Setup URL; încarcă cheia private key `.pem`; salvează; apasă Install și selectează repository-urile private dorite.</div>',
    '          </article>',
    '        </div>',
    '      </section>',
    '    </div>',
    '  </v-app-window>',
    '</v-app-instance>'
  ].join('');

  SYNO.ZionReleaseStation.Instance = Vue.extend({
    template: template,
    data: function () {
      return {
        tab: 'overview',
        health: {},
        healthState: 'checking',
        webAccessEnabled: true,
        webAccessState: 'loading',
        github: { mode: 'managed', configured: false, connected: false, installations: [], configuration_error: 'Zion Connector is not provisioned for this ReleaseStation instance' },
        githubConfig: { app_id: '', app_slug: '', setup_url: '' },
        githubConfigState: 'loading',
        githubMessage: ''
      };
    },
    computed: {
      healthClass: function () { return this.healthState; },
      healthTitle: function () {
        return this.healthState === 'healthy' ? 'ReleaseStation is healthy' : this.healthState === 'offline' ? 'ReleaseStation is unavailable' : 'Checking package health';
      },
      healthDetail: function () {
        return this.healthState === 'healthy' ? 'The local API and SQLite database are ready.' : 'Connect to the local ReleaseStation service to inspect package status.';
      },
      webAccessMessage: function () {
        if (this.webAccessState === 'loading') return 'Reading current setting…';
        if (this.webAccessState === 'saving') return 'Saving setting…';
        if (this.webAccessState === 'saved') return 'Setting saved.';
        if (this.webAccessState === 'error') return 'Unable to save or read the setting. Check the DSM web route.';
        return '';
      },
      githubChecks: function () {
        if (this.github.mode === 'managed') {
          return [
            { label: 'Zion managed connector available', ok: Boolean(this.github.configured) },
            { label: 'GitHub authorization complete', ok: Boolean(this.github.connected) },
            { label: 'Private repositories available', ok: Boolean(this.github.connected) }
          ];
        }
        return [
          { label: 'App credentials configured', ok: Boolean(this.github.configured) },
          { label: 'Private key uploaded', ok: Boolean(this.github.private_key_configured) },
          { label: 'GitHub installation connected', ok: Boolean(this.github.connected) }
        ];
      },
      githubSummary: function () {
        if (this.github.connected) return (this.github.installations || []).length + ' GitHub installation(s) connected. Private repositories selected in GitHub are available in the site wizard.';
        if (this.github.configuration_error) return this.github.configuration_error;
        return 'Completează configurarea și instalează GitHub App pentru acces la repository-uri private.';
      },
      githubMessageClass: function () {
        return this.githubConfigState === 'error' ? 'error' : this.githubConfigState === 'saved' ? 'ok' : '';
      }
    },
    mounted: function () {
      installStyles();
      this.loadWebAccess();
      this.checkHealth();
      this.loadGithub();
    },
    methods: {
      openWorkspace: function () {
        if (!this.webAccessEnabled) return;
        window.open(workspaceUrl(), '_blank');
      },
      loadWebAccess: function () {
        var self = this;
        fetch('/releasestation/api/v1/settings/web-access', { headers: { Accept: 'application/json' } })
          .then(function (response) { if (!response.ok) throw new Error('settings'); return response.json(); })
          .then(function (payload) {
            self.webAccessEnabled = Boolean(payload.data && payload.data.enabled);
            self.webAccessState = 'saved';
          })
          .catch(function () {
            self.webAccessEnabled = true;
            self.webAccessState = 'error';
          });
      },
      saveWebAccess: function () {
        var self = this;
        var requestedValue = self.webAccessEnabled;
        self.webAccessState = 'saving';
        fetch('/releasestation/api/v1/settings/web-access', {
          method: 'PUT',
          headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
          body: JSON.stringify({ enabled: requestedValue })
        })
          .then(function (response) { if (!response.ok) throw new Error('settings'); return response.json(); })
          .then(function (payload) {
            self.webAccessEnabled = Boolean(payload.data && payload.data.enabled);
            self.webAccessState = 'saved';
          })
          .catch(function () {
            self.webAccessEnabled = !requestedValue;
            self.webAccessState = 'error';
          });
      },
      loadGithub: function () {
        var self = this;
        fetch('/releasestation/api/v1/integrations/github', { headers: { Accept: 'application/json' } })
          .then(function (response) { if (!response.ok) throw new Error('github'); return response.json(); })
          .then(function (payload) {
            self.github = payload.data || {};
            self.githubConfig = { app_id: self.github.app_id || '', app_slug: self.github.app_slug || '', setup_url: self.github.setup_url || (window.location.origin + '/releasestation/api/v1/integrations/github/setup') };
            self.githubConfigState = 'saved';
          })
          .catch(function () { self.github = { mode: 'managed', configured: false, connected: false, installations: [], configuration_error: 'Zion Connector is not provisioned for this ReleaseStation instance' }; self.githubConfigState = 'error'; self.githubMessage = 'Zion Connector nu este provisionat pentru această instanță.'; });
      },
      saveGithubConfig: function () {
        var self = this;
        self.githubConfigState = 'saving';
        self.githubMessage = '';
        fetch('/releasestation/api/v1/integrations/github/config', { method: 'PUT', headers: { Accept: 'application/json', 'Content-Type': 'application/json' }, body: JSON.stringify(self.githubConfig) })
          .then(function (response) { return response.json().then(function (payload) { if (!response.ok) throw new Error((payload.error && payload.error.message) || 'Nu am putut salva configurarea GitHub App.'); return payload; }); })
          .then(function () { self.githubConfigState = 'saved'; self.githubMessage = 'Configurarea GitHub App a fost salvată. Încarcă acum cheia PEM.'; self.loadGithub(); })
          .catch(function (error) { self.githubConfigState = 'error'; self.githubMessage = error.message; });
      },
      uploadGithubKey: function (event) {
        var self = this;
        var file = event.target.files && event.target.files[0];
        if (!file) return;
        var form = new FormData();
        form.append('private_key', file);
        self.githubConfigState = 'saving';
        self.githubMessage = 'Uploading private key…';
        fetch('/releasestation/api/v1/integrations/github/private-key', { method: 'POST', headers: { Accept: 'application/json' }, body: form })
          .then(function (response) { return response.json().then(function (payload) { if (!response.ok) throw new Error((payload.error && payload.error.message) || 'Cheia PEM nu a putut fi încărcată.'); return payload; }); })
          .then(function () { self.githubConfigState = 'saved'; self.githubMessage = 'Cheia PEM a fost încărcată în directorul privat al pachetului.'; self.loadGithub(); })
          .catch(function (error) { self.githubConfigState = 'error'; self.githubMessage = error.message; });
      },
      installGithubApp: function () {
        var self = this;
        fetch('/releasestation/api/v1/integrations/github/install', { method: 'POST', headers: { Accept: 'application/json' } })
          .then(function (response) { return response.json().then(function (payload) { if (!response.ok) throw new Error((payload.error && payload.error.message) || 'GitHub App nu este configurată complet.'); return payload; }); })
          .then(function (payload) { window.open(payload.data.url, '_blank'); })
          .catch(function (error) { self.githubConfigState = 'error'; self.githubMessage = error.message; });
      },
      checkHealth: function () {
        var self = this;
        self.healthState = 'checking';
        fetch('/releasestation/api/v1/system/health', { headers: { Accept: 'application/json' } })
          .then(function (response) { if (!response.ok) throw new Error('health'); return response.json(); })
          .then(function (payload) { self.health = payload.data || {}; self.healthState = 'healthy'; })
          .catch(function () { self.health = {}; self.healthState = 'offline'; });
      }
    }
  });
})(window);
