# Changelog

Istoricul de mai jos păstrează evoluția proiectului și explică problema sau situația care a dus la fiecare schimbare. Commiturile istorice cu mesaje scurte sunt descrise pe baza fișierelor schimbate și a discuțiilor de implementare.

## Unreleased

- **Zion Connector Debug Console** — când `APP_DEBUG=1`, connectorul expune `/admin` cu filtrare, auto-refresh, expand pe rând și code coloring pentru headers/body. Sunt urmărite requesturile inbound de la Synology și GitHub, răspunsurile connectorului și requesturile outbound către GitHub, cu metode, statusuri și durate.
- **Debug safety** — logurile sunt persistente în MariaDB/SQLite, body-urile sunt limitate la 128 KiB, arhivele binare sunt omise, iar Authorization, cookies, token-uri, parole, secrets, private keys și pairing codes sunt redactate. Cu `APP_DEBUG=0`, consola și API-ul admin sunt dezactivate prin `404`.

- **Milestone 4 — Deployment Engine** — deployment-urile manuale și webhook sunt persistate mai întâi ca `queued`, apoi executate de worker-e cu lock per site și recovery la restart. Commiturile active sunt deduplicate, iar erorile de infrastructură sau de executor sunt înregistrate ca `failed` fără să lase job-uri blocate.
- **Pipeline și observabilitate** — fiecare deployment păstrează pașii `fetch`, `extract` și `publish`, durata, statusul și logurile build/deployment. Executorul transmite liniile scriptului în timp real prin SSE; interfața poate urmări coada, pașii și logurile fără input remote shell.
- **Dashboard live** — queue status separă deployment-urile `queued` de cele `running`, iar istoricul și median deploy time folosesc datele persistente reale. Manual Deploy răspunde cu `202 Accepted` și urmărește deployment-ul până la `deployed` sau `failed`.

- **Native DSM control center** — configurația nativă nu mai include conectarea GitHub. Overview-ul pune Runtime în partea de sus, afișează servicii verificate live și oferă acces direct la workspace, Activation și Configuration.
- **Workspace route validation** — Configuration permite editarea rutei DSM, normalizează slash-urile, respinge traversal/caractere nesigure și detectează conflicte cu rutele rezervate DSM. O schimbare de rută este marcată explicit ca necesitând reload-ul resursei nginx DSM.
- **Product naming** — suprafețele native, workspace-ul web, statusurile și metadatele de serviciu folosesc denumirea afișată „Release Station”.

- **GitHub push webhooks și auto-deploy** — Zion Connector primește webhook-uri publice, verifică HMAC `X-Hub-Signature-256`, deduplicatează delivery-urile și le asociază cu installation-ul clientului. SPK-ul face polling autentificat, filtrează strict repository-ul/installation-ul/branch-ul și `Push to deploy`, apoi rulează deploy-ul atomic cu trigger `webhook`. Aplicația nativă afișează starea webhook-ului, endpointul și numărul de evenimente acceptate.
- **Editor de deploy și verificări** — editorul scriptului folosește CodeMirror cu shell syntax highlighting și tema dark; au fost adăugate teste pentru cursorul webhook-ului, deduplicare și contractul connectorului.

- **Script atomic implicit și toolchain checks** — site-urile Atomic primesc automat un script revizuibil care pregătește `.current`, copiază release-ul în document root prin staging și rename atomic, iar scripturile personalizate salvate în Settings sunt executate cu variabilele `PROJECT_ROOT`, `CURRENT_DIR`, `RELEASE_DIR`, `WEB_ROOT`, `RELEASE_ID`, `DEPLOYMENT_ID` și `COMMIT_SHA`. Site-urile vechi primesc același script când sunt trecute pe Atomic.
- **System Overview configurabil** — Settings permite selectarea verificărilor pentru PHP, Composer, Node.js, npm, Git, rsync, unzip, tar, curl și clientul MariaDB/MySQL. Dashboard-ul rulează verificări reale pe NAS, afișează versiunea/calea detectată și Help Center oferă comenzile și pașii DSM pentru fiecare verificare.
- **Site history după adăugare/configurare** — după crearea unui site manual sau salvarea unui repository, aplicația deschide site-ul și încarcă imediat commiturile branch-ului configurat. Fiecare commit este comparat cu deployment history și apare ca deployed, failed sau not deployed.
- **Commit envelope Synology Connector** — endpointul connectorului răspunde cu { "commits": [...] }, nu cu un array direct. Clientul Go decodează acum formatul real și are test de regresie.
- **Taburi site** — ordinea este acum Overview, Deployments, Repository, Settings.
- **Notificări floating** — notificările de deployment sunt toast-uri fixe, centrate în viewport, independente de scroll, cu stare de succes/eroare, închidere manuală și timeout automat.
- **Numele aplicației GitHub** — textele explică faptul că utilizatorul se conectează prin aplicația GitHub Synology Connector, nu printr-o aplicație numită Zion și nu printr-o aplicație creată manual de client.
- **Favicon** — interfața web declară iconița pachetului ca favicon prin calea DSM /webman/3rdparty/zion-releasestation/images/app_64.png, eliminând cererea implicită la /favicon.ico.
- **Naming DSM** — etichetele afișate folosesc Zion Release Station, cu spațiu între Release și Station.

## Commit history

### 609424b — docs: add Zion ReleaseStation product specification

Definește produsul, obiectivele MVP, separarea dintre project root și document root, Web Station discovery, deploy-urile atomice, securitatea și direcția de packaging SPK.

### 5f191d6 — Added codex spec.md

Adaugă specificația operațională folosită ca referință pentru implementare, milestone-uri și limitele produsului.

### e76cb99 — feat: add M1 runtime and SPK foundation

Construiește runtime-ul Go, baza SQLite, structura SPK, serviciul local și primele endpoint-uri health.

### 4691042 — feat: add M1 ReleaseStation interface

Adaugă prima interfață web Vue pentru dashboard, Package Health, Runtime și controlul serviciului local.

### 15751f5 — fix: support explicit NAS SSH identity

Permite scripturilor NAS să folosească explicit cheia SSH configurată, în loc să depindă de cheia implicită.

### 3594d50 — fix: support sudo password for NAS deploy

Adaugă suport pentru parola sudo necesară instalării și verificărilor executate pe Synology.

### 120d5a1 — fix: avoid tty echo of NAS sudo password

Corectează modul de transmitere a parolei sudo pentru a evita afișarea sau ecoul ei în terminal.

### 9ddc594 — feat: add DSM native configuration surface

Adaugă suprafața de configurare nativă DSM pentru health, activation și configurarea inițială.

### 0fef9a0 — feat: replace DSM URL launcher with native app

Înlocuiește lansatorul simplu către URL cu o aplicație vizibilă în interiorul DSM și păstrează workspace-ul web ca fallback.

### d7bb8de — feat: add configurable DSM web workspace access

Adaugă opțiunea de activare/dezactivare pentru ruta web /releasestation/ și afișează setările relevante doar când ruta este activă.

### 9e5a0c7 — feat: deliver M2 sites and Web Station discovery

Adaugă catalogul de sites, filesystem discovery read-only, importul configurațiilor detectate și verificarea permisiunilor.

### 1c30719 — feat: add manual site wizard and GitHub connection

Adaugă wizard-ul pentru site-uri manuale, URL, project root, web root, framework detection, strategie și prima integrare GitHub.

### d89dc59 — feat: add GitHub App private repository connector

Adaugă suport pentru repository-uri private prin GitHub App, installation metadata și citirea repository-urilor acordate.

### a7cc258 — feat: configure GitHub App from native DSM UI

Adaugă configurarea GitHub App în aplicația nativă DSM, inclusiv câmpurile pentru App ID, slug, setup URL și cheia privată.

### 8d44145 — feat: add managed GitHub connector backend

Introduce connectorul managed care păstrează cheia GitHub App în serviciul public și livrează NAS-ului doar credentiale și metadata necesare.

### 22a329b — feat: add managed GitHub connection UI

Adaugă pairing-ul și starea conexiunii managed în interfața web și în fluxul de configurare.

### 1a5af1c — docs: specify Zion Connector implementation

Documentează arhitectura, endpointurile, pairing-ul, securitatea și separarea connectorului PHP de SPK.

### baed34e — feat: add managed PHP GitHub connector

Creează implementarea PHP a connectorului, cu GitHub client, sesiuni, installations și acces controlat la repository-uri.

### b3b40a4 — feat: deploy PHP connector with MariaDB

Adaugă structura și scriptul de deploy pentru ZionConnector pe Web Station/MariaDB.

### e627695 — chore: add connector deploy task

Adaugă task-ul VS Code și comanda standard pentru deploy doar a folderului ZionConnector.

### 5856c11 — fix: use configured SSH key in NAS tasks

Corectează task-urile pentru a folosi cheia SSH a utilizatorului wordpress-deploy și portul SSH configurat.

### f6e4bc2 — chore: restore connector deploy task

Restabilește task-ul de deploy după sincronizări și păstrează fluxul separat de deploy al SPK-ului.

### 5b8233c — chore: keep NAS example config local

Păstrează configurările locale de exemplu în afara commitului, astfel încât parolele și datele NAS să nu fie publicate.

### a01c932 — fix: deploy connector without preserving NAS metadata

Corectează copierea connectorului pe Synology pentru a nu transfera metadata și permisiuni incompatibile din filesystem-ul local.

### 57e1354 — fix: use Synology synopkg absolute path

Folosește calea absolută a binarului synopkg, deoarece PATH-ul shell-ului task-ului nu conținea utilitarele DSM.

### 4f3a25e — fix: resolve Go toolchain for WSL builds

Face build-ul backend-ului reproductibil în WSL prin configurarea toolchain-ului Go folosit de scripturi.

### 9e09801 — fix: configure connector for Web Station Nginx

Adaptează configurarea connectorului pentru reverse proxy/Web Station cu Nginx.

### dc208d8 — fix: load connector env under Web Station

Corectează încărcarea variabilelor de mediu atunci când PHP rulează prin Web Station, nu din shell-ul interactiv.

### 6fda1b5 — fix: grant Web Station runtime access to connector secrets

Acordă procesului Web Station accesul necesar la fișierele de configurare și secretele connectorului.

### 0f3dddb — fix: preserve remote deploy script quoting

Corectează quoting-ul scriptului de deploy transmis către Synology, inclusiv pentru valori cu caractere speciale.

### 7126143 — revert: use Apache 2.4 for connector

Revine la Apache 2.4 pentru connector după problemele de compatibilitate întâlnite pe configurația Web Station.

### 91b4b3f — feat: add automatic GitHub connector pairing

Adaugă sesiuni de pairing cu expirare și fluxul prin care clientul se conectează fără App ID, PEM sau PAT pe NAS.

### 6846a27 — feat: deploy authorized GitHub archives atomically

Adaugă download-ul arhivei autorizate prin connector, staging în .zion/releases, release history și activarea atomică prin symlink-ul current.

### 7bcbf82 — fix: open GitHub pairing from native DSM UI

Leagă butonul din aplicația nativă de fluxul public de pairing GitHub.

### 58775ea — fix: normalize connector return hosts

Normalizează hosturile și schemele folosite la return URL pentru a evita redirecturi greșite și hostname-uri invalide.

### 6e66a75 — feat: complete GitHub pairing through connector polling

Adaugă polling-ul din Release Station, schimbul codului de pairing și finalizarea conexiunii în workspace-ul clientului.

### 5c97866 — fix: allow GitHub pairing recovery

Permite recuperarea unei sesiuni de pairing după revenire sau repetarea fluxului fără a bloca instanța.

### 193ae3c — fix: complete workspace GitHub pairing status

Corectează sincronizarea stării finale între connector, backend, interfața web și aplicația nativă.

### 9e2f331 — Fix

Corectează endpointurile și testele pentru completarea conexiunii managed și pentru răspunsurile connectorului.

### 397a804 — Skip

Ignoră intrările rezervate din Web Station discovery, inclusiv directoarele metadata și recycle bin care nu sunt site-uri.

### e2a5021 — Polish

Îmbunătățește discovery-ul, permisiunile și prezentarea rezultatelor importului.

### e5b176a — Add

Adaugă istoricul deployment-urilor, logurile build/deployment, commit history, release records și integrarea download-ului de arhive GitHub.

### 255bbcc — Add

Ajustează interfața pentru afișarea și administrarea istoricului de deployment.

### f1d7f46 — Detect

Adaugă detectarea serverului HTTP Apache/Nginx din procese și fișiere de configurare, în locul etichetei generice Web Station.

### 15d8f10 — Improve

Adaugă dashboard metrics reale, queue status, median deploy time, verificări de servicii, repository grouping, branch loading și îmbunătățiri Web Station.

### 5c67043 — Add

Adaugă Settings per site, framework custom, tags, color, push-to-deploy configuration, deploy script, deployment retention, directoare copyable, migrarea SQLite și repară payload-ul Repository editor.

### fcf63a9 — Improve

Adaugă Help Center, linkuri Read more pentru System Overview, păstrează pagina selectată la refresh, încarcă commiturile imediat după configurarea repository-ului și introduce toast-uri floating pentru deployment.
