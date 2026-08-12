# Zion Connector

## Scop

Zion Connector este serviciul public Zion care face legătura dintre:

```text
ReleaseStation instalat pe Synology
             │ HTTPS + credential per instanță
             ▼
       Zion Connector
             │ GitHub App private key
             ▼
            GitHub
```

El permite utilizatorului final să apese **Connect GitHub** în aplicația ReleaseStation, să se autentifice în GitHub și să instaleze aplicația Zion în contul sau organizația sa. Utilizatorul nu trebuie să își creeze propria GitHub App și nu trebuie să încarce fișierul `.pem` pe NAS.

Cheia privată GitHub App rămâne exclusiv în Zion Connector. SPK-ul nu trebuie să conțină această cheie.

## Ce există deja în ReleaseStation

Clientul NAS este în:

```text
internal/githubconnector/client.go
```

Când sunt configurate următoarele variabile, ReleaseStation intră automat în modul `managed`:

```dotenv
RS_INSTANCE_ID=instance-issued-by-zion
RS_GITHUB_CONNECTOR_URL=https://connect.example.com
RS_GITHUB_CONNECTOR_TOKEN=credential-issued-for-this-instance
RS_PUBLIC_URL=https://nas.example.com
```

Dacă variabilele managed lipsesc, butonul rămâne vizibil, dar acțiunea va indica faptul că instanța nu este încă provisionată de serviciul Zion. Nu există fallback local cu App ID sau `.pem`.

## Pairing automat pentru clienții ReleaseStation

SPK-ul comercial nu primește App ID, client secret, `.pem` sau PAT. El cunoaște doar URL-ul connectorului, de exemplu `https://connect.example.com`.

1. ReleaseStation apelează `POST /pairing/sessions` fără credential și trimite `instance_id` plus URL-ul HTTPS de revenire.
2. Connectorul creează o sesiune de 10 minute și trimite utilizatorul la instalarea aplicației Zion în GitHub.
3. Callback-ul verifică OAuth-ul, utilizatorul și installation-ul GitHub, apoi revine în aplicație cu un cod de pairing de unică folosință.
4. ReleaseStation schimbă acel cod prin `POST /pairing/exchange` și salvează credentialul primit în `/var/packages/zion-releasestation/var/connector.json` cu `0600`.

Pentru development sau provisioning controlat pot fi setate explicit `RS_INSTANCE_ID` și `RS_GITHUB_CONNECTOR_TOKEN`, dar acestea nu trebuie livrate clienților. În fluxul comercial nu există configurare locală cu App ID sau `.pem`.

## Contractul HTTP obligatoriu

Toate endpoint-urile interne folosesc:

```http
Authorization: Bearer <instance credential>
Accept: application/json
```

Credential-ul trebuie să fie asociat cu `instance_id`. Nu accepta un token valid pentru instanța A pe ruta instanței B.

Răspunsurile de mai jos sunt JSON direct, fără wrapper `data`, deoarece acesta este formatul așteptat de `internal/githubconnector/client.go`.

### 1. Pornirea sesiunii de conectare

```http
POST /v1/instances/{instance_id}/github/sessions
Content-Type: application/json

{
  "return_url": "https://nas.example.com/releasestation/?github=connected"
}
```

Răspuns:

```json
{
  "id": "ghsess_01J...",
  "authorize_url": "https://github.com/apps/zion-releasestation/installations/new?state=...",
  "expires_in": 600
}
```

Reguli:

- generează `state` cu minimum 32 bytes aleatorii criptografic;
- păstrează doar hash-ul `state`, nu valoarea în clar;
- expirarea recomandată este 10 minute;
- leagă sesiunea de `instance_id` și de `return_url` validat;
- `authorize_url` trebuie să includă `state` URL-encoded;
- nu loga URL-ul complet, deoarece conține state-ul secret.

URL-ul GitHub este, în principiu:

```text
https://github.com/apps/{app_slug}/installations/new?state={state}
```

### 2. Callback-ul public GitHub

Configurează în GitHub App un callback public HTTPS, de exemplu:

```text
https://connect.example.com/github/callback
```

Pentru fluxul recomandat, activează autorizarea utilizatorului la instalare. GitHub va redirecționa utilizatorul către callback cu parametri de tipul:

```text
/github/callback?code=...&installation_id=123456&setup_action=install&state=...
```

Implementarea callback-ului trebuie să:

1. valideze `state` prin hash și expirare;
2. consume sesiunea atomically, astfel încât să nu poată fi reutilizată;
3. schimbe `code` pe un user access token GitHub;
4. verifice că `installation_id` aparține utilizatorului și GitHub App instalată;
5. verifice installation-ul cu autentificare ca GitHub App;
6. salveze installation-ul asociat cu `instance_id`;
7. marcheze sesiunea `connected`;
8. nu păstreze user access token-ul dacă nu este necesar după verificare;
9. redirecționeze către `return_url` numai după validarea completă.

Nu trata `installation_id` primit din URL ca fiind de încredere. GitHub documentează explicit că acest parametru poate fi falsificat; trebuie verificată asocierea dintre utilizator și installation. Vezi [About the setup URL](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/about-the-setup-url) și [Generating a user access token](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-user-access-token-for-a-github-app).

Dacă este folosit un setup URL fără OAuth la instalare, păstrează totuși verificarea state + verificarea installation-ului și tratează acest flux ca fallback. Pentru produsul comercial, callback-ul OAuth este preferat deoarece permite verificarea utilizatorului care a aprobat instalarea.

Răspunsul final al callback-ului poate fi:

```http
303 See Other
Location: https://nas.example.com/releasestation/?github=connected
```

Nu permite redirect către un URL arbitrar. `return_url` trebuie să fie validat la provisionarea instanței sau să aparțină unei liste de domenii autorizate pentru acea instanță.

### 3. Statusul conexiunii

```http
GET /v1/instances/{instance_id}/github/status
```

Răspuns deconectat:

```json
{
  "state": "disconnected",
  "installations": [],
  "message": "No GitHub installation is connected."
}
```

Răspuns conectat:

```json
{
  "state": "connected",
  "account_login": "customer-org",
  "installations": [
    {
      "github_installation_id": 123456,
      "account_login": "customer-org",
      "account_type": "Organization",
      "repository_selection": "selected",
      "permissions": {
        "contents": "read",
        "metadata": "read"
      }
    }
  ]
}
```

Stări recomandate:

```text
disconnected
pending
connected
suspended
error
```

### 4. Repository-urile accesibile

```http
GET /v1/instances/{instance_id}/github/repositories
```

Răspuns:

```json
{
  "repositories": [
    {
      "installation_id": 123456,
      "account_login": "customer-org",
      "id": 987654,
      "name": "private-site",
      "full_name": "customer-org/private-site",
      "private": true,
      "default_branch": "main",
      "clone_url": "https://github.com/customer-org/private-site.git",
      "ssh_url": "git@github.com:customer-org/private-site.git"
    }
  ]
}
```

Pentru fiecare installation:

1. generează un JWT al GitHub App folosind cheia privată;
2. cere un installation access token;
3. folosește `GET /installation/repositories` cu tokenul temporar;
4. paginează rezultatele, maximum 100 pe pagină;
5. unește rezultatele și elimină duplicatele după `installation_id + repository_id`;
6. nu persistă installation token-ul în baza de date.

Tokenurile de instalare expiră după o oră. Cache-ul în memorie poate fi folosit pentru aproximativ 50–55 de minute, cu regenerare automată înainte de expirare. [Documentația GitHub pentru installation access tokens](https://docs.github.com/en/rest/apps/apps#create-an-installation-access-token-for-an-app).

### 5. Branch-urile unui repository

```http
GET /v1/instances/{instance_id}/github/repositories/{owner}/{repo}/branches?installation_id=123456
```

Răspuns:

```json
{
  "branches": ["main", "develop", "staging"]
}
```

Verifică următoarele înainte de request:

- `owner` și `repo` trebuie să fie segmente simple, fără traversal;
- `installation_id` trebuie să aparțină instanței autentificate;
- repository-ul trebuie să fie prezent în lista accesibilă installation-ului;
- nu accepta un installation ID arbitrar doar pentru că este numeric.

### 6. Arhiva pentru deploy atomic

```http
GET /v1/instances/{instance_id}/github/repositories/{owner}/{repo}/archive?installation_id=123456&ref=main
Authorization: Bearer <instance credential>
```

Connectorul verifică faptul că installation-ul aparține instanței și că repository-ul este în lista acordată de GitHub App, apoi descarcă arhiva tar.gz cu token temporar de installation. Tokenul GitHub nu ajunge pe NAS; ReleaseStation primește numai bytes-ii arhivei și îi extrage într-un release local.

Pentru strategia `atomic`, document root-ul site-ului trebuie să fie:

```text
/volume1/www/example/current -> .zion/releases/<release-id>
/volume1/www/example/.zion/releases/<release-id>
```

ReleaseStation pregătește release-ul, păstrează site-ul pe versiunea curentă și înlocuiește symlink-ul `current` prin rename atomic. Arhivele cu path traversal, symlink-uri sau fișiere speciale sunt respinse, iar dimensiunea arhivei și a rezultatului expandat este limitată la 512 MB.

## Configurarea GitHub App

Creează aplicația sub o organizație Zion, nu sub un cont personal, pentru a putea controla accesul și rotația cheilor.

### Setări recomandate

```text
Visibility: Public
Repository permissions:
  Contents: Read-only
  Metadata: Read-only
User authorization during installation: Enabled
Callback URL: https://connect.example.com/github/callback
Webhook: Enabled după implementarea evenimentelor de lifecycle
```

Pentru checkout Git prin HTTPS este necesară permisiunea `Contents`. Nu cere permisiuni de write, Actions, Administration sau Organization decât dacă o funcție concretă le folosește. [Choosing permissions for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/choosing-permissions-for-a-github-app).

Aplicația trebuie să fie publică pentru ca alte conturi și organizații să o poată instala. Utilizatorul poate alege `Only select repositories`, ceea ce limitează accesul App-ului la repository-urile selectate. [Sharing your GitHub App](https://docs.github.com/en/apps/sharing-github-apps/sharing-your-github-app).

### Secretele GitHub App

Păstrează în secret manager sau în variabile de environment ale connectorului:

```dotenv
CONNECTOR_GITHUB_APP_ID=123456
CONNECTOR_GITHUB_APP_SLUG=zion-releasestation
CONNECTOR_GITHUB_CLIENT_ID=Iv1....
CONNECTOR_GITHUB_CLIENT_SECRET=...
CONNECTOR_GITHUB_PRIVATE_KEY_PATH=/run/secrets/zion-github-app.pem
CONNECTOR_PUBLIC_BASE_URL=https://connect.example.com
```

Permisiuni recomandate:

```text
private key: 0600
secret directory: 0700
service account: non-root
```

Nu pune aceste valori în GitHub, în SPK, în `README.md`, în loguri sau în payload-urile returnate către NAS.

## Modelul de date minim

Poți folosi PostgreSQL în serviciul public. SQLite este suficient pentru development, dar nu este alegerea recomandată pentru connectorul multi-client.

### `connector_instances`

```text
id                  text primary key
license_id          text nullable
credential_hash     text not null
status              text not null default 'active'
return_host         text not null
created_at          timestamp not null
updated_at          timestamp not null
last_seen_at        timestamp nullable
```

`credential_hash` trebuie să fie hash-uit cu SHA-256/HMAC sau un KDF potrivit. Nu salva credential-ul NAS în clar.

### `github_connect_sessions`

```text
id                  text primary key
instance_id         text not null
state_hash          text unique not null
return_url          text not null
status              text not null default 'pending'
expires_at          timestamp not null
github_installation_id bigint nullable
error_code          text nullable
created_at          timestamp not null
consumed_at         timestamp nullable
```

Indexuri:

```text
index(state_hash)
index(instance_id, status)
index(expires_at)
```

### `github_installations`

```text
id                         text primary key
instance_id                text not null
github_installation_id     bigint not null unique
account_login              text not null
account_type               text not null
repository_selection       text not null
permissions_json           jsonb not null
suspended_at               timestamp nullable
created_at                 timestamp not null
updated_at                 timestamp not null
```

Un installation GitHub poate fi legat de o singură instanță Zion. Dacă modelul comercial permite migrarea unei instalări între instanțe, implementează o procedură explicită de transfer și audit, nu suprascrie automat asocierea.

## Structura recomandată a codului

Poți implementa serviciul într-un repository separat sau într-un director separat care nu este inclus în SPK:

```text
zion-connector/
├── cmd/connector/main.go
├── internal/config/
├── internal/httpapi/
│   ├── auth.go
│   ├── github.go
│   ├── callback.go
│   └── health.go
├── internal/github/
│   ├── client.go
│   ├── jwt.go
│   └── oauth.go
├── internal/store/
│   ├── instances.go
│   ├── sessions.go
│   └── installations.go
├── migrations/
├── Dockerfile
└── README.md
```

Nu copia implementarea în procesul ReleaseStation de pe NAS. NAS-ul trebuie să rămână clientul connectorului, nu să devină proprietarul cheii GitHub App.

## Fluxul complet de runtime

```text
1. Native UI / web UI
       │ POST /integrations/github/install
       ▼
2. ReleaseStation
       │ POST /v1/instances/{id}/github/sessions
       ▼
3. Zion Connector
       │ creează state + URL GitHub
       ▼
4. ReleaseStation deschide authorize_url
       ▼
5. Utilizatorul se autentifică în GitHub
       │ instalează Zion App + selectează repository-uri
       ▼
6. GitHub -> /github/callback
       │ code + installation_id + state
       ▼
7. Zion Connector validează userul și installation-ul
       │ persistă asocierea instance_id -> installation_id
       ▼
8. Redirect către RS_PUBLIC_URL/releasestation/?github=connected
       ▼
9. ReleaseStation face polling pe /github/status
       ▼
10. Wizard-ul citește /github/repositories și /branches
```

## Autentificarea NAS -> Connector

Credential-ul per instanță se provisionază după activarea licenței sau în timpul setup-ului inițial. Connectorul trebuie să ofere endpoint intern de provisionare, protejat separat:

```text
POST /internal/instances
Authorization: Bearer <connector-admin-token>

{
  "instance_id": "instance-issued-by-zion",
  "license_id": "license-123",
  "return_host": "nas.example.com"
}
```

Răspunsul credential-ului se afișează o singură dată către componenta de activare. Pentru rotație:

1. emite un credential nou;
2. acceptă temporar vechiul și noul credential;
3. actualizează `config.env` pe NAS;
4. verifică `/github/status`;
5. revocă vechiul credential.

Nu permite utilizatorului să introducă manual un connector token în UI. Acesta este un secret de infrastructură și trebuie provisionat de fluxul de licențiere.

## Webhook-uri GitHub recomandate

După ce fluxul inițial funcționează, activează webhook-ul GitHub App:

```text
POST /github/webhook
X-Hub-Signature-256: sha256=...
X-GitHub-Event: installation / installation_repositories
```

Evenimente utile:

```text
installation.created
installation.deleted
installation.suspend
installation.unsuspend
installation_repositories.added
installation_repositories.removed
```

Verifică HMAC-ul pe body-ul brut cu `crypto/subtle.ConstantTimeCompare`. Webhook-ul nu trebuie să logheze body-ul complet și nu trebuie să accepte modificări de permisiuni fără revalidare prin GitHub API.

## Securitate obligatorie

- HTTPS obligatoriu pentru API și callback.
- Private key doar pe serverul connectorului, cu permisiuni `0600`.
- Niciun PAT, user token sau installation token în loguri.
- State criptografic aleator, hash-uit și single-use.
- Expirare și cleanup pentru sesiuni abandonate.
- Verificare strictă `instance_id` + credential.
- Allowlist pentru `return_url`; fără open redirect.
- Rate limit pentru pornirea sesiunilor și callback-uri.
- Timeouts la toate request-urile către GitHub.
- Cache temporar în memorie pentru installation tokens, niciodată în DB.
- Audit log fără secrete: instanță, account GitHub, installation ID, acțiune, rezultat.
- Verificare periodică a installation-urilor suspendate sau șterse.
- Rotație planificată pentru GitHub private keys și connector credentials.
- Proces non-root și secret manager pentru deployment.

## Testare

### Unit tests

Testează separat:

- generarea și expirarea state-ului;
- consumarea single-use a sesiunii;
- hash și comparație credential;
- validarea `return_url`;
- parsing-ul callback-ului;
- schimbul OAuth code -> user token;
- verificarea installation-ului;
- cache-ul și expirarea installation token-ului;
- paginarea repository-urilor;
- validarea owner/repository;
- HMAC webhook;
- suspendarea și ștergerea installation-ului.

### Integration tests

Folosește `httptest.Server` pentru un GitHub API fals și verifică:

1. `POST /github/sessions` returnează URL și state;
2. callback-ul valid salvează installation-ul;
3. callback-ul cu state expirat este respins;
4. callback-ul cu state reutilizat este respins;
5. callback-ul cu installation care nu aparține App-ului este respins;
6. tokenul nu apare în response sau log;
7. repository-urile private apar după instalare;
8. endpoint-urile unei alte instanțe răspund cu `401` sau `403`;
9. redirect-ul către host neautorizat este respins;
10. suspendarea installation-ului schimbă statusul în `suspended`.

### Contract test cu ReleaseStation

Rulează ReleaseStation cu:

```dotenv
RS_INSTANCE_ID=test-instance
RS_GITHUB_CONNECTOR_URL=http://127.0.0.1:...
RS_GITHUB_CONNECTOR_TOKEN=test-credential
RS_PUBLIC_URL=https://nas.example.com
```

Verifică toate cele patru request-uri pe care le face clientul NAS și formatul JSON exact documentat mai sus.

## Deployment recomandat

1. Creează GitHub App publică în organizația Zion.
2. Configurează callback-ul HTTPS și permisiunile minime.
3. Publică Zion Connector pe un VPS sau în infrastructura existentă de licențiere.
4. Pune TLS în reverse proxy și lasă serviciul Go să asculte doar local.
5. Configurează PostgreSQL și backup-uri criptate.
6. Încarcă private key-ul prin secret manager.
7. Rulează migrations.
8. Creează un instance credential pentru NAS-ul de development.
9. Scrie variabilele managed în `/var/packages/zion-releasestation/var/config.env`, cu `0600`.
10. Repornește SPK-ul și verifică `/releasestation/api/v1/integrations/github`.
11. Testează instalarea GitHub pe un repository privat de test.
12. Activează webhook-urile și monitorizarea după validarea fluxului de bază.

Exemplu de `config.env` pe NAS:

```dotenv
RS_INSTANCE_ID=dev-nas-001
RS_GITHUB_CONNECTOR_URL=https://connect.example.com
RS_GITHUB_CONNECTOR_TOKEN=<provisioned-secret>
RS_PUBLIC_URL=https://nas.example.com
```

## Criterii de acceptare

Implementarea este gata când:

- clientul creează sesiune din butonul nativ și din web UI;
- utilizatorul poate instala Zion App în cont personal sau organizație;
- poate selecta repository-uri private;
- callback-ul verifică userul, state-ul și installation-ul;
- ReleaseStation vede `connected` fără configurare `.pem`;
- wizard-ul afișează repository-urile și branch-urile permise;
- un token al instanței nu poate vedea datele altei instanțe;
- cheia privată nu ajunge pe NAS;
- installation token-urile nu sunt persistate;
- revocarea/suspendarea din GitHub este reflectată în status;
- toate testele unit, integration și contract trec;
- serviciul poate roti cheia GitHub App și credential-urile fără downtime semnificativ.

## Referințe GitHub

- [About the setup URL](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/about-the-setup-url)
- [Sharing your GitHub App](https://docs.github.com/en/apps/sharing-github-apps/sharing-your-github-app)
- [Choosing permissions for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/choosing-permissions-for-a-github-app)
- [Generating a user access token](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-user-access-token-for-a-github-app)
- [Generating an installation access token](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app)
- [REST API endpoints for GitHub Apps](https://docs.github.com/en/rest/apps/apps)
