# ciexecutor-jenkins

The ciexecutor provider for Jenkins: a capability provider of `numinance.ports.ciexecutor.v1` over the Jenkins remote access API
(`/api/json`, `buildWithParameters`, the queue, `crumbIssuer`). It holds no credential (a handle for the intent `ci.jenkins`, sent to
the egress proxy, which injects the Jenkins user name and API token) and starts only a job the customer already defines.

Plugin id: `dev.numinance.ciexecutor-jenkins` (replay build: `dev.numinance.ciexecutor-jenkins-replay`). The id that
`numi init --ci jenkins` now writes into the generated Jenkins binding. Repositories initialised by v0.2.0 carry the earlier
placeholder `dev.numinance.official/jenkins`, which is not a valid plugin id (a plugin id is dot separated labels,
`manifest.proto`); they must edit it by hand or re-run init with `--force`.

## Configuration

One launch argument, recorded by the operator's install beside the manifest: `-url https://host[:port][/context]`. It is the only
server the provider may reach. https only, a lowercase DNS name (no IP literal, no wildcard), no user name, query or fragment, port
1 to 65535 except 80 (the egress proxy re-originates every port but 80 over TLS). The manifest's one network permission is
`host:port` from it; nothing a request carries can change the destination. The replay build uses the made-up `jenkins.example.test`.

### How the live manifest is produced, and the pending ruling

There is no static, pre-signed manifest file for the live build. The manifest is built **in memory at launch**: `main` parses `-url`,
and `ghprovider.New` assembles the `Manifest` (capabilities, one `network_intents` entry `{host, port}` taken from `-url`, the
credential intent `ci.jenkins`), which `GetManifest` returns in the handshake. Its `digest` is the sha256 of the running executable and
`signer_identity` is the fixed string `https://numinance.dev/official|<plugin id>`. Nothing signs a per-customer manifest: the build
artifact is signed once, and the host and port in the manifest differ per customer because the launch argument does.

The master design (section 17, extensions) makes `network_intents[]{host, port}` part of the signed manifest, while a Jenkins host is
per customer. That is an open question shared with the Jira ticketing provider, covered by the proposed ADR 0060 (an opus ruling is
pending). **Live mode of this plugin is gated on that ruling**: until it lands, do not install the live build as an official, signed
plugin, and do not read this README as saying its manifest is signed per customer. The replay build declares no network or credential
permission and is unaffected.

## Credentials and egress

- The provider asks the host for a credential handle for `ci.jenkins` and sends it in the `Numinance-Handle` header to the egress
  proxy, which strips it and injects the HTTP Basic credential (Jenkins user + API token) the broker holds. No token, user name or
  `Authorization` header is ever in this process, a log or a cassette. The manifest declares no `materialized` credential.
- The transport sends nothing without a handle the host gave a proxy for, and nothing to any host:port other than the configured one;
  it never falls back to a direct connection and never follows a redirect.
- Broker catalog entry (the operator's, not shipped here): intent `ci.jenkins`, delivery header `Authorization`, destination the
  configured host and port.
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
4. A run is found by that parameter, first in the queue (`/queue/api/json`) and then in the job's recent builds (the 100 builds
   Jenkins lists), before a new one is started. The queue is global (every job of the server) and the operation id is not secret, so
   a queue item counts only when **both** hold: it carries our `numinance_operation_id`, and it is an item of our job. The task URL
   of Jenkins is `<root URL>/job/<a>/job/<b>`, and its root part is Jenkins's own setting, which behind a reverse proxy may differ
   from the path of `-url`. So the item is ours when its URL path **ends with** our job path
   (`/job/<owner>/job/<name>/job/<pipeline>`) and what precedes that is the configured context path of `-url`, or a root mount that
   has no `/job/` segment. A path that does not end with ours is another job and the item is skipped: never adopted, never an error, so
   it can neither shadow the real run nor block a call; a cancel only ever uses the id of our own item. A path that ends with ours
   but has a `/job/` segment before it can not be told from a parent folder (or from a root mount that contains `/job/` and is not
   the configured one), so it fails closed with `UPSTREAM_ERROR` and nothing is started. With no task URL the task's short name is
   compared; with neither, the item is not ours. A repeat under the same key therefore starts no second run (`existing: true`); a
   window of 100 builds that lacks the operation fails closed (`LIMIT_EXCEEDED`) and starts nothing. The revision reported is the
   parameter of the build, checked against the grammar.
5. Anyone who can build the job can set its parameters. The job's permissions are the customer's to restrict to the pipeline identity;
   the provider can not tell a build of its own from one that carries the same parameters.

## Calls

- `StartOperation` (HIGH, DEPLOY side effect, keyed; declares and emits one `ci-run-ref` receipt): checks the job, finds a run of the key,
  asks for a crumb, `POST .../buildWithParameters` with the parameters in a urlencoded body (never in the URL), requires `201` with a
  `Location` of a queue item and answers `QUEUED`. Dry run checks the job and the parameters and starts nothing. Jenkins can not check
  that a commit exists (that is the SCM's), so a dry run does not.
- `GetOperation`: queue item -> `QUEUED`; build -> `RUNNING` (building, or no result yet) or the end state; `started_at` from the
  build timestamp, `finished_at` as timestamp + duration.
- `CancelOperation` (MEDIUM; declares `ci-action`): a queued run is removed with `POST /queue/cancelItem?id=`, a build is stopped with
  `POST .../<n>/stop`, then re-read (up to three looks); an ended run is answered as it is and nothing is posted. If Jenkins has not
  reported an end by then, the state answered is the one read (`RUNNING`), never `CANCELLED`.
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

- `ciexecutor-jenkins` (live, through the egress proxy; `-url` required) and `ciexecutor-jenkins-replay` (`cmd/replay`, over
  `replay/cassettes/ci.json`). The replay build's manifest declares no network and no credential and the `dev` environment only.
- The cassette is hand-written from the Jenkins documentation, **not recorded**: the server is `jenkins.example.test`, names and the
  crumb are made up, and it keeps only the answer headers `Location` and `Content-Length`; `ParseCassette` refuses any other header
  and a test scans the file for hosts and secret patterns. Exchanges with the same method, path (query compared as values) and form
  answer in order, the last repeating.
- `go test ./plugins/official/ciexecutor-jenkins/... ./conformance/ciexecutor/...` from `numinance-spec`. The ciexecutor conformance
  kit (CORE-001 to CORE-008) runs against the replay build in `conformance/ciexecutor/v1/kit_test.go`
  (`TestKit_ReplayingJenkinsProvider`). The unit tests run against a small stateful fake Jenkins.

## Verified against what

Replay only. Nothing here has run against a real Jenkins. **A live run is owed.** Facts used from the Jenkins remote access API
documentation, to confirm on a live server (verify against a live Jenkins):

- `POST /job/<name>/buildWithParameters` answers `201` with `Location: <root>/queue/item/<id>/`; a job with no parameters answers 400.
- `GET /job/<name>/api/json?tree=...` honors `tree` with a range (`builds[...]{0,100}`), `property[parameterDefinitions[name]]`, and
  `artifacts[relativePath]` on a build; queue items list `actions[parameters[name,value]]` and `task[url]`.
- `POST /queue/cancelItem?id=` and `POST /job/<name>/<n>/stop` answer a redirect (2xx or 3xx is accepted); the stop becomes `ABORTED`
  asynchronously.
- With an API token over Basic auth Jenkins does not need a session for a crumb; the provider fetches and sends one anyway, and does
  not keep cookies. A server that ties crumbs to a session ("proxy compatibility" off) may refuse it: then a build request answers
  403 (PERMISSION_DENIED).
- A queue item that was cancelled leaves no trace in the lists read here, so a repeat of `CancelOperation` or a `GetOperation` after it
  answers `NOT_FOUND` (the first cancel answers `CANCELLED`). Jenkins's `/queue/item/<id>/api/json` shows `cancelled` for a while;
  the id is not part of the operation id, so it is not used.
- `building`, `result` (`null` while building), `timestamp` (start, ms) and `duration` (ms) of a build.
- Large queues or job folders are only bounded by the 4 MiB answer limit.

## Known limits (left as they are)

- Evidence emission failures are discarded (`_ = Emit`), as in the scm provider; the host turns a declared record that is missing into
  PARTIAL.
- After a successful queue cancel, a full 100-build window can make the re-read fail `LIMIT_EXCEEDED` (it fails closed).
- If the build request times out after Jenkins accepted it, the side effect is not recorded (`Wrote`); the host's retry finds the run by
  the key and starts no second one.
- A dry run over an existing run answers no operation id and emits no `ci-run-ref` receipt (a dry run has no effect to record).
- The crumb field name must look like Jenkins's (contain `crumb`) and must not begin with a header name the client or the proxy owns.
- The host must be a DNS name; a name whose labels are all numbers (decimal, or `0x` followed by any hex digits including none, as
  `0x7f.0x0.0x0.0x1`) is refused. This is not an SSRF control: the egress proxy denies private, loopback, link-local and metadata
  ranges, and that is to be verified in the live card.
- Ambiguity: an item that carries our operation id and whose task URL ends with our job path but has a `/job/` segment before it
  (a job of the same name in a parent folder, or a root mount containing `/job/` that is not the configured context path) is an
  `UPSTREAM_ERROR` for start, get and cancel while it sits in the queue; a server mounted at a path with `/job/` in it must give that
  path in `-url`. Someone who can queue such an item can therefore block calls on that operation (never start a second run).
- Same-job forgery: someone who can build THIS job can set `numinance_operation_id` and `numinance_revision` on a build of it and so
  be taken for the run. That is Jenkins write access to the job and is the customer's to restrict (point 5).
- A gap exists in which Jenkins has removed an item from the queue but the build is not yet in the job's builds list; a lookup in that
  gap sees neither, and a retry under the same key could POST a second build. It is narrow, and it is not new.
- A crumb field name with a non-ASCII character is refused (case folding is only defined for ASCII).

## Not done

- A live run against a real Jenkins and a recording tool (a server and an API token are needed).
- Inclusion in the release bundle (`.goreleaser.yaml`, `tools/release/bundle`): not added in this card.
- Live mode is gated on the ruling about a per-customer network intent in a signed manifest (ADR 0060, above).
- Multibranch pipelines, branch jobs whose name contains `/`, a sha256 digest of artifacts, `TIMED_OUT`.
- The provider does not fetch or return the console log and does not read pipeline stages.
