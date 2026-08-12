# Zion ReleaseStation

Zion ReleaseStation este un manager self-hosted de build și deployment pentru Synology DSM: un „Laravel Forge” orientat către NAS, distribuit ca pachet nativ `.spk` și accesibil printr-o interfață web integrată în DSM.

Repository-ul păstrează numele `zion-release-station`. În specificația produsului, numele canonic este **Zion ReleaseStation**, package ID-ul este `zion-releasestation`, iar binarul runtime va fi `zion-releasestation`. „Zion Deploy” a fost numele de lucru folosit în discuțiile inițiale.

> Documentație de produs și arhitectură. Implementarea efectivă a componentelor este următorul pas al proiectului.

## Obiectiv

ReleaseStation conectează repository-uri Git cu site-uri găzduite în Web Station și execută release-uri controlate direct pe Synology:

```text
GitHub / GitLab / Git
          │
          ▼
   Zion ReleaseStation
          │
   queue + secure worker
          │
          ▼
  Synology filesystem
          │
          ▼
      Web Station
```

Produsul trebuie să fie suficient de general pentru Laravel, Symfony, WordPress/WooCommerce, Flarum, aplicații PHP, site-uri statice, Node/Vite și alte proiecte Git configurabile.

## Direcția MVP Professional

MVP-ul trebuie să includă:

- setup inițial și autentificare administrator;
- dashboard cu proiecte, starea site-urilor și deployment-urile recente;
- conectare repository Git, branch și SSH deploy keys;
- testarea conectivității Git și known-host verification;
- import/discovery pentru site-urile Web Station;
- detectarea framework-ului și a document root-ului;
- deploy manual și deploy prin webhook GitHub/GitLab;

### GitHub connector managed pentru clienți

Fluxul recomandat pentru produsul livrat este connectorul Zion managed. Clientul apasă **Connect GitHub** în aplicația nativă, se autentifică în GitHub, instalează aplicația Zion în contul sau organizația sa și selectează explicit repository-urile permise. Clientul nu creează o GitHub App proprie și nu încarcă o cheie `.pem` pe NAS.

SPK-ul comunică prin HTTPS cu serviciul connector Zion. Cheia privată a aplicației GitHub rămâne exclusiv în serviciul Zion; NAS-ul primește doar metadata și credențiale temporare necesare pentru operațiile autorizate. Provisionarea este făcută de serviciul de licențiere/connector, nu din interfața utilizatorului:

```env
RS_INSTANCE_ID=instance-issued-by-zion
RS_GITHUB_CONNECTOR_URL=https://connect.example.com
RS_GITHUB_CONNECTOR_TOKEN=provisioned-instance-credential
RS_PUBLIC_URL=https://nas.example.com
```

Connectorul managed folosește endpoint-urile `/v1/instances/{instance_id}/github/sessions`, `/status`, `/repositories` și `/repositories/{owner}/{repo}/branches`. Dacă aceste variabile nu sunt provisionate, ReleaseStation păstrează modul self-hosted avansat.

Serviciul nu păstrează PAT-uri și nu expune installation token-ul prin API. Tokenul temporar este folosit pentru citirea repository-urilor și expiră automat.
- queue de deployment cu lock per proiect și deduplicarea commit-urilor depășite;
- pași configurabili prin `deployment.yml`;
- loguri structurate și streaming live al progresului;
- deployment in-place și atomic releases;
- health checks după activare și rollback automat la eșec;
- istoric persistent, retenție de release-uri, rollback manual și audit log;
- management de environment/secrets fără returnarea valorilor salvate în API;
- build reproductibil `.spk`, validare și artefacte de release.

### Pairing GitHub pentru pachetul livrat

În pachetul livrat clientului este necesar doar URL-ul public al connectorului. La prima apăsare pe **Connect GitHub**, ReleaseStation generează automat un `instance_id`, deschide pairing-ul public și primește credentialul numai după autorizarea aplicației Zion în GitHub.

Credentialul este salvat în directorul runtime DSM (`connector.json`, mod `0600`), nu în SPK și nu în setările editabile din UI. Cheia `.pem`, App ID-ul, client secret-ul și installation token-urile rămân exclusiv în connectorul Zion.

```env
RS_GITHUB_CONNECTOR_URL=https://connect.example.com
RS_PUBLIC_URL=https://nas.example.com
```

### Fluxul unui deployment

```text
Webhook / Deploy manual
          ↓
Validare semnătură și branch
          ↓
Queue + project lock
          ↓
Fetch / checkout release nou
          ↓
Pași configurabili de build
          ↓
Health check
          ↓
Switch atomic sau activare in-place
          ↓
Istoric, audit și notificare
```

Pentru atomic deployment, site-ul rulează în continuare release-ul curent până când următoarea versiune a fost construită și verificată:

```text
/volume1/www/example/
├── current -> releases/<release-id>
├── releases/
└── shared/
    ├── .env
    └── storage/
```

## Integrarea Web Station

Integrarea Web Station este parte din MVP, nu doar o extensie ulterioară. Utilizatorul trebuie să poată descoperi site-urile existente și să importe, unde este posibil:

- domeniul sau portalul;
- document root-ul;
- project/deployment root-ul;
- tipul serviciului și backend-ul HTTP;
- profilul și versiunea PHP;
- porturile HTTP/HTTPS;
- reverse-proxy target-ul.

Arhitectura folosește un `WebStationAdapter` cu capability detection. Web Station WebAPI este metoda principală; citirea configurației locale este doar fallback read-only. Nu se vor ghici endpoint-uri DSM: contractul API trebuie verificat pe versiunea reală de Web Station prin request-urile făcute de interfața DSM.

Pentru aplicații precum Laravel, diferența dintre document root (`.../public`) și project root (`.../`) trebuie detectată și păstrată explicit.

## Arhitectură tehnică

Runtime-ul final trebuie să fie compact și independent de stack-ul instalat pe NAS:

| Zonă | Alegere |
| --- | --- |
| Backend/API/worker | Go, un singur proces |
| Frontend | Vue 3, TypeScript, Vite |
| UI | Tailwind CSS, PrimeVue, VueUse, Lucide |
| Animații și interacțiuni | `@vueuse/motion`, SortableJS |
| Editor de pipeline | Monaco Editor, lazy-loaded |
| Loguri live | xterm.js, read-only în MVP |
| Database | SQLite, migrații versionate |
| Git și transport | Git CLI, OpenSSH, SSH ED25519 |
| Secrets | master key locală + AES-GCM |
| Licențiere | `LicenseProvider` și entitlements semnate Ed25519 |
| Pachet | Synology `.spk` |

Node, PHP, Laravel și Docker sunt unelte de development/build, nu dependențe obligatorii ale runtime-ului SPK. Binarul Go trebuie compilat pentru `linux/amd64` cu `CGO_ENABLED=0`, iar frontend-ul trebuie livrat ca asset static, ideal embedded în aplicație.

## Target Synology și build

Ținta inițială este:

```text
DSM >= 7.2.2 (test target curent: DSM 7.4-90075)
DS1019+
CPU family: x86_64
build platform: apollolake
SPK architecture: x86_64
```

Mediul recomandat este Windows + WSL2 Ubuntu + Docker, cu Synology Package Toolkit izolat în container. Laptopul construiește și validează pachetul; NAS-ul rămâne mediul de instalare și testare reală.

Comanda de produs dorită este:

```bash
make spk
```

Rezultatul va fi un artefact de forma:

```text
dist/zion-releasestation-<version>-apollolake.spk
```

Fluxul CI planificat este:

```text
git push
  → Go tests
  → frontend tests/build
  → linux/amd64 build
  → SPK packaging
  → package validation
  → artifact / GitHub Release
```

## Interfața utilizator

UI-ul este o funcție principală a produsului și trebuie să fie o experiență premium, nu un template generic de admin. Direcția vizuală combină claritatea Linear/Vercel cu feedback-ul din GitHub Actions:

- dashboard cu pipeline vizual și statusuri live;
- proiecte organizate prin carduri și filtre multi-select;
- onboarding cu „Discover Web Station Sites”;
- command palette;
- drag & drop pentru pașii pipeline-ului;
- Monaco pentru scripturi și `deployment.yml`;
- terminal/log viewer live, read-only;
- diff de release, health status și acțiuni de rollback vizibile;
- dark mode, animații discrete și stări explicite pentru loading/success/failure.

Interfața nu va expune un shell interactiv în browser în MVP.

## Model de securitate

ReleaseStation trebuie să ruleze cu privilegii reduse și să fie fail-safe:

- daemon-ul nu rulează ca root; pachetul folosește modelul DSM `run-as: package`;
- parolele, token-urile, cheile SSH și webhook secrets nu se păstrează plaintext;
- cheia privată SSH rămâne pe NAS și nu este returnată după salvare;
- verificarea host-urilor SSH este obligatorie;
- webhook-urile verifică HMAC și branch-ul așteptat;
- input-ul HTTP nu este executat direct ca shell;
- scripturile din repository sunt executate doar printr-un contract de deployment aprobat și auditat;
- aplicația verifică permisiunile filesystem înainte să accepte un proiect;
- nu se modifică direct configurații interne DSM dacă există un mecanism oficial;
- expirarea licenței nu oprește site-urile, nu șterge configurația și nu blochează rollback-ul release-urilor existente.

Datele mutabile aparțin directorului de runtime DSM, nu directorului de instalare al pachetului:

```text
/var/packages/zion-releasestation/var/
├── releasestation.db
├── master.key
├── git/keys/
├── logs/deployments/
├── locks/
└── runtime/
```

## Non-goals pentru v1

Nu fac parte din MVP:

- DNS, emitere Let's Encrypt sau administrare firewall DSM;
- provisioning MariaDB sau alte baze de date;
- Docker orchestration/Kubernetes;
- server provisioning generic;
- mail server;
- shell interactiv remote în browser;
- RBAC multi-user complex;
- modificarea bidirecțională a configurației Web Station înainte ca discovery read-only să fie stabilă;
- dependența de Internet pentru pornirea aplicației sau executarea deployment-urilor deja autorizate.

## Structură planificată

```text
zion-release-station/
├── cmd/releasestation/
├── internal/
│   ├── api/ auth/ audit/ config/ crypto/
│   ├── database/ deploy/ git/ licensing/
│   ├── projects/ releases/ secrets/ webhooks/ webstation/
│   └── system/
├── frontend/
├── synology/
│   ├── conf/ scripts/ WIZARD_UIFILES/ ui/ nginx/
├── build/synology/
├── scripts/
├── .github/workflows/
├── .vscode/
├── Makefile
├── SECURITY.md
├── CONTRIBUTING.md
└── README.md
```

## Principii de implementare

1. Build-ul local și CI trebuie să producă același tip de artefact.
2. Runtime-ul SPK trebuie să fie independent de PHP, Node și Docker.
3. Orice integrare DSM/Web Station trebuie izolată prin adaptoare și capability detection.
4. Deployment-urile trebuie să fie observabile, reversibile și protejate de lock-uri.
5. Site-urile existente nu trebuie afectate de expirarea licenței sau de un deployment eșuat.
6. Testele, migrațiile, validarea SPK și auditul fac parte din feature, nu sunt lucrări ulterioare.

## Licențiere planificată

Arhitectura trebuie să permită ulterior licențe Pro/Business cu activare per NAS, număr de site-uri și feature entitlements. Clientul Go verifică local lease-ul semnat și poate funcționa cu lease offline și grace period; serverul de licențiere nu trebuie contactat la fiecare deployment.

## Status

Acest repository începe cu documentația de produs derivată din conversația de proiect. Următoarele livrabile sunt scheletul monorepo, build-ul reproducibil, integrarea SPK de bază și apoi implementarea incrementală a backend-ului, frontend-ului și Web Station adapter.
