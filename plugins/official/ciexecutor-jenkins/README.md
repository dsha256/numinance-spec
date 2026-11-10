# ciexecutor-jenkins

The ciexecutor provider for Jenkins: a capability provider of `numinance.ports.ciexecutor.v1` over the Jenkins remote access API
(`/api/json`, `buildWithParameters`, the queue, `crumbIssuer`). It holds no credential (a handle for the intent `ci.jenkins`, sent to
the egress proxy, which injects the Jenkins user name and API token) and starts only a job the customer already defines.

Plugin id: `dev.numinance.ciexecutor-jenkins` (replay build: `dev.numinance.ciexecutor-jenkins-replay`). The id that
`numi init --ci jenkins` now writes into the generated Jenkins binding. Repositories initialised by v0.2.0 carry the earlier
placeholder `dev.numinance.official/jenkins`, which is not a valid plugin id (a plugin id is dot separated labels,
`manifest.proto`); they must edit it by hand or re-run init with `--force`.

## Configuration

One launch argument, recorded by the operator's install beside the manifest: `-url https://host[/context]`. It is the only server the
provider may reach. `ParseURL` accepts https only, printable ASCII with no percent sign or backslash, no user name, query or
fragment, port absent or 443 (nothing else: the manifest's one network permission is the host on 443), a lowercase DNS name of two
or more labels (each at most 63 bytes, the name at most 253) that is no address in any spelling (decimal, octal, hex, short dotted,
IPv6), no wildcard, trailing dot or non-ASCII character, and a final label that is not a local, private or reserved space
(`localhost`, `local`, `internal`, `localdomain`, `lan`, `intranet`, `home`, `corp`, `arpa`, `invalid`, `onion`: a private server is
named by a fully qualified name of the customer's own domain). The context path has at most four plain segments. Nothing a request
carries can change the destination. The replay build and the tests use the made-up `jenkins.example.test`.
This is a grammar, not an SSRF control by itself: the egress proxy decides what a name resolves to and refuses loopback, link-local,
metadata and (without signed customer policy for that exact host) private addresses.

### How the live manifest is produced

There is no static, pre-signed manifest file for the live build. The manifest is built **in memory at launch**: `cmd/live` parses
`-url`, and `ghprovider.New` assembles the `Manifest` (capabilities, one `network_intents` entry `{host, 443}` taken from `-url`, the
credential intent `ci.jenkins`), which `GetManifest` returns in the handshake. Its `digest` is the sha256 of the running executable and
`signer_identity` is the fixed string `https://numinance.dev/official|<plugin id>`. Nothing signs a per-customer manifest: the build
artifact is signed once, and the host in the manifest differs per customer because the launch argument does.

The master design (section 17) makes `network_intents[]{host, port}` part of the signed manifest, while a Jenkins host is per
customer. The master is silent on that. The narrowest reading is recorded in `docs/build/decisions.md` 2026-11-09/ciexecutor-jenkins-
network-intent: the existing mechanism, no `plugin/v1` change, and the live build **installed by the operator against one named
server, not shipped in an official signed bundle**. The direction of ADR 0060 OQ2 (an instance-bound flag on `NetworkIntent`, the host
bound only in org-scoped signed policy) stays owed before an official install. The replay build declares no network or credential
permission.

## Credentials and egress

Rows: `docs/build/decisions.md` 2026-11-09/ciexecutor-jenkins-network-intent and -credentials.

- The provider asks the host for a credential handle for `ci.jenkins` and sends it in the `Numinance-Handle` header to the egress
  proxy, which strips it and injects the HTTP Basic credential (Jenkins user + API token) the broker holds. No token, user name or
  `Authorization` header is ever in this process, a log or a cassette (a test asserts that no request leaves it with `Authorization`,
  `Cookie` or `Proxy-Authorization`). The manifest declares no `materialized` credential.
- The transport sends nothing without a handle the host gave a proxy for, and nothing to any host:port other than the configured one;
  it never falls back to a direct connection and never follows a redirect. It also sends only what the provider makes: GET, HEAD and
  POST for a path under `<context>/job/`, `<context>/queue/` or `<context>/crumbIssuer/`, with no dot segment and no escaped dot,
  slash, backslash or NUL. The proxy's injection rule cannot restrict a path inside the host, so the script console, credentials and
  user management are out of reach of the provider by this rule, beside the account's own permissions.
- Broker catalog entry (the operator's, not shipped here): intent `ci.jenkins`, `DeliveryHeader`, header `Authorization`, prefix
  `Basic `, empty encoding (not `EncodingBasic`, whose user is fixed to `x-access-token`), `SecretRef` naming a secret whose value is
  the base64 of `user:token`, `ReadClass` false, `RuleBound` false, one destination `{configured host, 443, path prefix = the context
  path of -url when there is one}`.
- The Jenkins account is dedicated to the pipeline folder and holds Overall/Read, Job/Read, Job/Build and Job/Cancel only, never
  Job/Configure, Job/Create, Credentials or Administer.
- Every client has a 30 s timeout and the caller's deadline; an answer is read through a 4 MiB limit and an oversized one is
  `LIMIT_EXCEEDED`, never decoded; a request body is at most 1 MiB. The body of an error answer is dropped (it is Jenkins's HTML).

## What a pipeline must do

Jenkins has no idempotency key and a build request answers with a queue item that does not outlive the queue. So:

1. The repository `<owner>/<name>` is a Jenkins folder path, and the pipeline is a job under it: the job is
   `/job/<owner>/job/<name>/job/<pipeline segment>...`. (A GitHub Organization folder with one folder per repository has this shape.)
   A multibranch job, a branch name with `/` and a folder that is itself the pipeline are not supported.
2. The job is a parameterized job that declares the string parameters `numinance_revision` (the full commit id to build) and
   `numinance_operation_id`, and every parameter the profile passes (names `numinance_*` from a caller are refused). The provider
   checks this before it starts anything and fails `FAILED_PRECONDITION` (reason `CONFLICT`) when it does not hold or the job is
   disabled. The job must check out `numinance_revision` itself, and its configuration must be the customer's: a candidate must not
   be able to change the job definition (the same rule as a protected default branch for GitHub Actions).
3. The operation id the provider issues is `<32 hex of sha256(key, repository, pipeline)>.<pipeline with '/' as ':'>`, at most
   128 bytes (so a pipeline name is at most 95 bytes). It names the job because `GetOperation` carries only the repository and the id.
4. A run is found by that parameter, first in the queue (`/queue/api/json`) and then in the job's recent builds (the newest
   1000, read newest first in pages of 100), before a new one is started. The queue is global (every job of the server) and the operation id is not secret, so
   a queue item counts only when **both** hold: it carries our `numinance_operation_id`, and it is an item of our job. The task URL
   of Jenkins is `<root URL>/job/<a>/job/<b>`, and its root part is Jenkins's own setting, which behind a reverse proxy may differ
   from the path of `-url`. So the item is ours when its URL path **ends with** our job path
   (`/job/<owner>/job/<name>/job/<pipeline>`) and what precedes that is the configured context path of `-url`, or a root mount that
   has no `/job/` segment. A path that does not end with ours is another job and the item is skipped: never adopted, never an error, so
   it can neither shadow the real run nor block a call; a cancel only ever uses the id of our own item. A path that ends with ours
   but has a `/job/` segment before it can not be told from a parent folder (or from a root mount that contains `/job/` and is not
   the configured one), so it fails closed with `UPSTREAM_ERROR` and nothing is started. With no task URL the task's short name is
   compared; with neither, the item is not ours. A repeat under the same key therefore starts no second run (`existing: true`); the
   newest 1000 builds, if they lack the operation, fail closed (`LIMIT_EXCEEDED`) and start nothing. The revision reported is the
   parameter of the build, checked against the grammar.

   **The gap between the queue and the build list.** Jenkins takes an item off the queue a moment before the build shows in the job's
   list, and keeps the item at `/queue/item/<id>/api/json` for a few minutes with the build it became (`executable`) or `cancelled`.
   The operation id does not carry the item id, so the provider remembers it, per operation, in memory (at most 1024, oldest dropped)
   when it starts a run or sees it in the queue. When neither list shows a run whose item is remembered, it reads the item: it must be
   an item of our job that carries our operation id; if it has an `executable`, that build is read and answered; if it is neither
   cancelled nor a build yet, the run is `QUEUED` and can be looked up, repeated under its key (no second POST) and cancelled; if it is
   `cancelled`, unknown (404) or not ours, the run is absent as before. The memory is a hint: a provider process that never saw the
   item (restarted since) has only the two lists, and the gap is then the narrow one described under Known limits.
5. Anyone who can build the job can set its parameters. The job's permissions are the customer's to restrict to the pipeline identity;
   the provider can not tell a build of its own from one that carries the same parameters.

## Calls

- `StartOperation` (HIGH, DEPLOY side effect, keyed; declares and emits one `ci-run-ref` receipt): checks the job, finds a run of the key,
  asks for a crumb, `POST .../buildWithParameters` with the parameters in a urlencoded body (never in the URL), requires `201` with a
  `Location` of a queue item and answers `QUEUED`. Dry run checks the job and the parameters and starts nothing. Jenkins can not check
  that a commit exists (that is the SCM's), so a dry run does not.
  **Outcome of the build request.** `Wrote` (the DEPLOY side effect) is claimed only for a `201` with a queue item, or for a run that
  Jenkins shows under the key after an answer that did not say (a timeout, a connection reset, a 5xx from a proxy in front, a 2xx
  without the queue item): the provider then looks the run up by its key, once, and answers it as a new run. A refusal (4xx, 3xx) is
  not looked up. If the look finds nothing, or cannot be made (the caller's deadline has passed), the call fails with the reason of
  the error (`TIMEOUT`, `UPSTREAM_UNAVAILABLE`, `UPSTREAM_ERROR`) and claims no effect: **those reasons on a build request mean the
  outcome is not known**, and the host's retry under the same key finds the run if there is one and starts no second.
- `GetOperation`: queue item -> `QUEUED`; build -> `RUNNING` (building, or no result yet) or the end state; `started_at` from the
  build timestamp, `finished_at` as timestamp + duration.
- `CancelOperation` (MEDIUM; declares `ci-action`): a queued run is removed with `POST /queue/cancelItem?id=`, a build is stopped with
  `POST .../<n>/stop`, then re-read (up to three looks); an ended run is answered as it is and nothing is posted. If Jenkins has not
  reported an end by then, the state answered is the one read (`RUNNING`), never `CANCELLED`. After a queue cancel, a re-read that
  shows the run in neither list (also when all 1000 builds are read and the last page is still full: a run that had left the queue first would head the first
  page, being the newest build) is the cancel Jenkins confirmed, `CANCELLED`; a run in the gap is read from its item (see above) and, once
  a build, stopped by the next look.
- Evidence: the `ci-run-ref` receipt of a start (also of a repeat, so a retry heals a missing one) and the `ci-action` receipt of a
  cancel are submitted after the effect. If the host does not take one, the call still ends `SUCCEEDED` with the effect recorded (a
  `Wrote` for the DEPLOY or WRITE side effect), no evidence id for that record, and a `WARNING` diagnostic (`UPSTREAM_UNAVAILABLE`,
  `Call.Warn`); the host derives `PARTIAL` from the missing declared evidence (a provider never claims `PARTIAL`). The failure is not
  discarded and not turned into a failure of a call whose effect has happened.
- `ListArtifacts`: names from `artifacts[relativePath]`, sizes from the `Content-Length` of a `HEAD` of each artifact (the build JSON
  has no size; content is never fetched; a missing length is `UPSTREAM_ERROR`, never a guess). The digest is empty: Jenkins reports no
  sha256 (its fingerprints are MD5). At most 256 artifacts.
- `GetLogReference`: `https://<configured server><context>/job/.../<n>/console`, built from the configuration and never from the
  `url` Jenkins answers (its root URL setting is not a fact the provider relies on). A queued run has no log: `CONFLICT`.

CSRF: `GET /crumbIssuer/api/json` before every `POST`; a 404 means crumbs are off and none is sent. The crumb and its field name are
checked against a narrow grammar (they become a header) and never logged or stored.

State mapping: `SUCCESS` -> SUCCEEDED; `ABORTED` -> CANCELLED; `FAILURE`, `UNSTABLE`, `NOT_BUILT` and any result this code does not
know -> FAILED (an unstable build is not a success); building or no result -> RUNNING; in the queue -> QUEUED. Jenkins reports a timeout
(Pipeline `timeout`, the build-timeout plugin) as `ABORTED`, so a timeout reads as CANCELLED and **TIMED_OUT is never produced**.
Status mapping: 401/403 -> PERMISSION_DENIED, 404 -> NOT_FOUND, 429 -> RATE_LIMITED, 5xx and connection errors -> UPSTREAM_UNAVAILABLE,
400 on a build request -> CONFLICT, a deadline -> TIMEOUT, an answer that breaks the grammar or is not JSON -> UPSTREAM_ERROR.

Untrusted input: job names, parameters, results, artifact paths and queue/build JSON are data. Repository, pipeline, revision,
parameter names (`^[a-z][a-z0-9_]{0,63}$`) and values (at most 1024 printable bytes, at most 32) are validated before the first
request; an artifact path with an empty, `.` or `..` segment, a backslash, a control character, over 256 bytes or repeated is refused.
Nothing from Jenkins is logged; the provider logs nothing.

## Builds, cassettes, tests

- `ciexecutor-jenkins` (`cmd/live`: live, through the egress proxy; `-url` required) and `ciexecutor-jenkins-replay` (`cmd/replay`, over
  `replay/cassettes/ci.json`). The replay build's manifest declares no network and no credential and the `dev` environment only.
- The cassette is hand-written from the Jenkins documentation, **not recorded**: the server is `jenkins.example.test`, names and the
  crumb are made up, and it keeps only the answer headers `Location` and `Content-Length`; `ParseCassette` refuses any other header
  and a test scans the file for hosts and secret patterns. Exchanges with the same method, path (query compared as values) and form
  answer in order, the last repeating.
- `go test ./plugins/official/ciexecutor-jenkins/... ./conformance/ciexecutor/...` from `numinance-spec`. The ciexecutor conformance
  kit (CORE-001 to CORE-008) runs against the replay build in `conformance/ciexecutor/v1/kit_test.go`
  (`TestKit_ReplayingJenkinsProvider`). The unit tests run against a small stateful fake Jenkins.
- The same kit (all eight cases, none skipped) also runs against the **live build** in `provider/live_kit_test.go`
  (`TestLive_ConformanceKitPassesThroughTheProxyToATLSFake`): the provider's `Live` transport sends absolute-form requests with a
  handle to a fake egress proxy on loopback, which checks the destination (`jenkins.example.test:443`), strips the handle, injects a
  credential only it holds and re-originates over TLS to a fake Jenkins that serves a certificate for the reserved name. The
  fake Jenkins refuses a request without the injected credential or a POST without the crumb, so the test also proves the crumb is
  sent, the handle never reaches Jenkins, the provider holds no authenticating header, every request is for the instance host on 443
  and the build request and the stop are the only POSTs. Only the proxy of the test dials a loopback listener for the reserved name.

## Verified against what

Replay and a local TLS fake only. Nothing here has run against a real Jenkins. **A live run is owed** (see Live runs). Facts used from the Jenkins remote access API
documentation, to confirm on a live server (verify against a live Jenkins):

- `POST /job/<name>/buildWithParameters` answers `201` with `Location: <root>/queue/item/<id>/`; a job with no parameters answers 400.
- `GET /job/<name>/api/json?tree=...` honors `tree` with a range (`builds[...]{0,100}`, then `{100,200}` and so on up to `{900,1000}`, newest build first, end exclusive; a
  page shorter than the range is the end of the list), `property[parameterDefinitions[name]]`, and
  `artifacts[relativePath]` on a build; queue items list `actions[parameters[name,value]]` and `task[url]`.
- `POST /queue/cancelItem?id=` and `POST /job/<name>/<n>/stop` answer a redirect (2xx or 3xx is accepted); the stop becomes `ABORTED`
  asynchronously.
- With an API token over Basic auth Jenkins does not need a session for a crumb; the provider fetches and sends one anyway, and does
  not keep cookies. A server that ties crumbs to a session ("proxy compatibility" off) may refuse it: then a build request answers
  403 (PERMISSION_DENIED).
- A queue item that was cancelled leaves no trace in the lists read here, so a repeat of `CancelOperation` or a `GetOperation` after it
  answers `NOT_FOUND` (the first cancel answers `CANCELLED`). `/queue/item/<id>/api/json` shows `cancelled` or the `executable` of
  the build for a few minutes after the item leaves the queue, with `id`, `task[name,url]` and `actions[parameters[name,value]]`
  (the gap lookup relies on this: confirm it, and for how long, on a live server).
- `building`, `result` (`null` while building), `timestamp` (start, ms) and `duration` (ms) of a build.
- Large queues or job folders are only bounded by the 4 MiB answer limit. A page of 100 builds with parameters is read through the same
  limit.
- The container fixture (see Live runs) additionally relies, unchecked, on `ApiTokenStore.addFixedNewToken` taking a plain token of
  `11` and 32 hex characters, and on `Folder.createProject` in an init script.

## Live runs

Two harnesses exist. **Neither has been run**: no Docker daemon was running and no controller was available where they were
written. The live-fake conformance run (`TestLive_ConformanceKitPassesThroughTheProxyToATLSFake`) stays the only executed
evidence for the live build.

- `provider/live_controller_test.go` (`TestLiveController_DryRunAndOptionalBuild`): against an operator's controller, through the
  operator's egress proxy, which holds the Jenkins token. The test holds no credential, only a handle. Skipped unless
  `NUMINANCE_JENKINS_LIVE=1`; then `NUMINANCE_JENKINS_LIVE_{URL,PROXY,HANDLE,REPO,PIPELINE,REVISION}` are all required. It does a dry
  run of `StartOperation` (checks the job and its `numinance_*` parameters, starts nothing). Because starting a build is a side effect,
  the steps that start, repeat under the same key, wait to leave the queue (2 minutes), cancel, read the end state and read the console
  reference run only with `NUMINANCE_JENKINS_LIVE_START=1`. It never logs a title, a parameter or log content. The env parsing and the
  scenario logic are covered by unit tests that run (`TestControllerEnv`, `TestLiveScenario_AgainstTheFake`).
- `provider/live_docker_test.go` (`//go:build integration`, `TestDockerJenkins_StartRepeatRunCancel`): builds the throwaway controller
  of `testdata/jenkins-docker` (image pinned by digest, `cloudbees-folder`, setup wizard off, an init script that creates an admin with
  a random per-run API token, no anonymous read, crumbs on, folders `acme/widgets` and the jobs `release` and `dry` with the
  `numinance_*` and `environment` parameters, each sleeping 300 s). It drives the provider for `jenkins.example.test` through a plain
  HTTP in-test proxy that checks the destination and handle, strips the handle and injects Basic credentials from the random token. It
  expects QUEUED, a repeat under the same key (same operation id, no second build), RUNNING within 2 minutes, a console reference,
  no artifacts, and `CANCELLED`. It compiles and passes `go vet -tags integration`, and is skipped unless `NUMINANCE_JENKINS_DOCKER=1`
  and `docker info` succeeds:
  `cd numinance-spec && NUMINANCE_JENKINS_DOCKER=1 go test -tags integration -run TestDockerJenkins ./plugins/official/ciexecutor-jenkins/provider/`.
  Expect to fix the fixture on the first run.

## Known limits (left as they are)

- **A job with more than 1000 builds fails a new key with `LIMIT_EXCEEDED` when the run is not among the newest 1000.** A start looks
  for the key newest first in up to 10 pages of 100 builds and stops at the first page of fewer than 100 (the end of the list). If the
  tenth page is still full and lacks the key, an earlier attempt could exist further back, so the call fails closed and starts nothing.
  A key retried after more than 1000 newer builds therefore cannot start a run. The pages are separate requests: a build that lands
  between two of them shifts the list down, so an entry can appear on two pages (harmless) but none is skipped. Up to 10 requests
  are made for a get or cancel of a run that does not exist, and a page that holds more builds than the range asked for is an
  `UPSTREAM_ERROR`.
- If the build request fails in a way that does not say whether Jenkins took it and the look-up by key finds nothing, the call fails
  `TIMEOUT` or `UPSTREAM_UNAVAILABLE` with no effect claimed (see Calls): the caller must treat those reasons on a start as
  "unknown" and retry under the same key. The side effect of a stop request that times out is not claimed either (a stop is
  repeatable).
- A dry run over an existing run answers no operation id and emits no `ci-run-ref` receipt (a dry run has no effect to record).
- The crumb field name must look like Jenkins's (contain `crumb`) and must not begin with a header name the client or the proxy owns.
- The host grammar (see Configuration) refuses addresses in any spelling and local or private final labels. It does not resolve
  names: the egress proxy denies loopback, link-local, metadata and (without signed customer policy for that exact host) private
  ranges, and that is to be verified in the live card.
- Ambiguity: an item that carries our operation id and whose task URL ends with our job path but has a `/job/` segment before it
  (a job of the same name in a parent folder, or a root mount containing `/job/` that is not the configured context path) is an
  `UPSTREAM_ERROR` for start, get and cancel while it sits in the queue; a server mounted at a path with `/job/` in it must give that
  path in `-url`. Someone who can queue such an item can therefore block calls on that operation (never start a second run).
- Same-job forgery: someone who can build THIS job can set `numinance_operation_id` and `numinance_revision` on a build of it and so
  be taken for the run. That is Jenkins write access to the job and is the customer's to restrict (point 5).
- The queue item memory is per process. A provider process restarted between a start and a lookup of a run that is in the gap between
  the queue and the build list sees neither list and answers `NOT_FOUND`, and a start under the same key in that moment could POST a
  second build. The gap is seconds long and needs a restart inside it.
- A crumb field name with a non-ASCII character is refused (case folding is only defined for ASCII).

## Not done

- A live run against a real Jenkins and a recording tool (a server and an API token are needed). Two harnesses are written and not run
  (see Live runs).
- Inclusion in the release bundle (`.goreleaser.yaml`, `tools/release/bundle`): not added in this card.
- An official signed install: the instance-bound network intent (ADR 0060 OQ2) is owed; the live build is operator-installed.
- Multibranch pipelines, branch jobs whose name contains `/`, a sha256 digest of artifacts, `TIMED_OUT`.
- The provider does not fetch or return the console log and does not read pipeline stages.
