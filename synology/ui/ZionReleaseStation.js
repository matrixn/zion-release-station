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
    '.zion-native-card{padding:17px;background:#fff;border:1px solid #d9e0e7;border-radius:7px;box-shadow:0 2px 5px #1e33420b}',
    '.zion-native-card h2{margin:0 0 14px;font-size:14px;font-weight:600}.zion-native-card p{margin:0;color:#687482;font-size:12px;line-height:1.55}',
    '.zion-native-status{display:flex;align-items:center;gap:9px;margin-bottom:13px;font-weight:600}.zion-native-dot{width:9px;height:9px;border-radius:50%;background:#e0a326;box-shadow:0 0 0 4px #e0a3261c}.zion-native-dot.healthy{background:#1aa36f;box-shadow:0 0 0 4px #1aa36f1c}.zion-native-dot.offline{background:#d64b4b;box-shadow:0 0 0 4px #d64b4b1c}',
    '.zion-native-list{margin:0}.zion-native-list div{display:flex;justify-content:space-between;gap:20px;padding:8px 0;border-top:1px solid #edf0f3}.zion-native-list dt{color:#77828d}.zion-native-list dd{margin:0;text-align:right;font-weight:600}',
    '.zion-native-toggle{display:flex;align-items:flex-start;gap:10px;margin:2px 0 8px;cursor:pointer}.zion-native-toggle input{width:16px;height:16px;margin:1px 0 0;accent-color:#1677d2}.zion-native-toggle strong,.zion-native-toggle small{display:block}.zion-native-toggle strong{font-size:12px}.zion-native-toggle small{margin-top:3px;color:#77828d;font-size:11px;line-height:1.4}.zion-native-setting-state{margin:0 0 15px 26px;color:#77828d;font-size:11px}',
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
        webAccessState: 'loading'
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
      }
    },
    mounted: function () {
      installStyles();
      this.loadWebAccess();
      this.checkHealth();
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
