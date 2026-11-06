// The sandboxprovider port (ADR 0006 as amended by ADR 0033): creates, runs, snapshots and destroys the one sandbox of one run of one work unit on the runner's host. One sandbox per run: a work unit's later run gets a new sandbox, restored from a snapshot.
// Callers: only the sandbox context in the runner calls this port, under the runner identity; the host refuses every other actor kind. The port is never a model tool, and no tool_groups or scopes are declared. The provider runs on the runner's host with authority over mounts, so only a provider bound to the runtime deployment serves it; the host refuses one installed from the registry. The worktree, artifact and output directories are host directories; the host reads them itself.
// Authority: nothing a provider reports is authority. The host builds every SandboxSpec from the run's grant, the policy decision and the plan, and refuses to dispatch a request whose change_id, work_unit_id or run_id differs from the ActionContext, whose deadline is later than the ActionContext deadline, or whose profile is below the floor minimum (compared through the floors' explicit table, never by number) or the work unit's ceiling. ADR 0033's ceiling bounds the ladder level and risk class of the work unit, checked before Create; the port's verbs act inside an admitted work unit, and the ceiling never lowers the risk class decided for an action (an Exec under PROCESS stays PRIVILEGED). The provider creates exactly the requested profile or refuses; it never downgrades. Every answer is a provider claim: the host recomputes the digests of the files it reads, and after a restore the tree digests of both directories against the ones Snapshot returned, and treats a mismatch as a provider fault; that a capture matched the run's state rests on the deployment-bound provider.
// Isolation, under every profile: the image is the one the runner holds under image_digest and is never pulled; the environment holds only the Exec env and TRACEPARENT, nothing of the host environment; the run's processes run as an unprivileged OS user dedicated to the run, which can read no runner, broker, egress proxy, provider or other run state, no host credential (cloud, SSH agent, git, instance role) and no canary. The host's worktree holds no credential: its git configuration has no credential helper, token-bearing URL or extra header. Under CONTAINER and GVISOR also: read-only root filesystem, writable tmpfs, all capabilities dropped, seccomp, no-new-privileges, a user namespace, the limits of SandboxLimits, and no host path but the worktree and artifact directories; GVISOR adds the user-space kernel. Their write model: at Create the provider hands every entry at or under allowed_paths, and the artifact tree, to the run's uid as in the PROCESS Create handover, so whatever the run creates or changes is owned on disk by the run's uid, never through a mapping to the host's user; every other entry stays host-owned and read-only for the run. An allowed path missing at Create is writable only under another allowed path, so the host refuses a request with an allowed path that neither exists nor lies under another (allow the directory to create a file in it). Their network namespace reaches only egress_proxy, which is unique to the run (the proxy binds it to this sandbox); every other address, DNS included, is unreachable, and with no egress_proxy there is no network. PROCESS is a direct child process: freeze, kill and Destroy act on every process of the run's OS user, never only on a process tree; the run gets a private temporary directory as TMPDIR; the worktree, artifact and TMPDIR roots are 0700 and owned by the run's user, under a work root no other run's user can traverse, and at Create the provider hands the host-created trees to the run's user without following links, with modes normalized as at restore and the Create rules for ACLs, extended attributes, labels and file flags (Destroy comment); Destroy removes every file the user owns outside the worktree and artifact directories, which the host keeps, and the user is never reused. Under every profile, neither the run's user nor its numeric uid is reused while any directory it owns is kept, which after a complete Destroy handover is none, so this binds only while a Destroy is unfinished. Host paths readable by the run's user and the host network are reachable, egress_proxy must be empty, the provider enforces the deadline and the limits the OS supports, and a provider that cannot run each run as its own OS user refuses PROCESS. The host offers PROCESS only on a host where, at each Create, an active probe of the metadata addresses, made as the run's user, fails and no cloud credential source is present, and the decision for an Exec under PROCESS counts its egress as unclassified, so PRIVILEGED. Under every profile the run's user can connect to no runner, broker, egress proxy or container runtime socket other than its own proxy, and those endpoints authenticate their caller.
// Paths: worktree paths follow the path rules of the authorexecutor port (file comment, Paths); overlap checks and masking compare them under NFC and Unicode simple case folding. Under CONTAINER and GVISOR only allowed_paths are writable, enforced on the resolved path after following symlinks, and hardlink creation is refused; every other worktree path, .numinance/context/ included (the host neither allows nor masks it), is read-only. Each masked path is absent: its content is not readable and nothing can be created at or under it. Under GVISOR the provider also refuses to create, at any depth, an entry whose name case-folds to .git, .gitmodules, .gitattributes or .lfsconfig. Under PROCESS the sandbox enforces no worktree path rule: the host leaves every masked path out of the worktree it creates, so it is absent on disk, and its candidate check covers the rest. A masked path absent on disk (PROCESS, or after a restore) is never a deletion: the candidate takes it from the run's base (authorexecutor port, Paths). Whatever a profile does not enforce in the sandbox, and floor patterns under every profile, the host's check of the candidate tree enforces (authorexecutor port, Paths). Host paths are absolute, '/'-separated and printable, with no '.' or '..' segment, resolved with no symlink component under the provider's configured work root (FAILED_PRECONDITION); one equal to, above or under a path of a live sandbox, or one that held a directory of a destroyed run, is refused (FAILED_PRECONDITION): every run gets new, never-used worktree, artifact and output directories. The host creates the trees with no non-directory of more than one link (a clone copies objects, never hardlinks them) and every entry owned by its own user; under every profile the provider refuses Create (FAILED_PRECONDITION) of a tree holding either. The last segment of each host path is the run_id, which keys one sandbox, so reuse is detectable without state; the provider's refusal of a destroyed run's path is best-effort within its key retention, and the host's check (HOST-001) is the durable one. A grammar breach, a last segment other than run_id, or an overlap within one spec is INVALID_ARGUMENT.
// Output: the provider writes each exec's standard output and error to exec/<exec_seq>/stdout and exec/<exec_seq>/stderr under output_path, a host directory never mounted in the sandbox and, with its parents, neither readable nor writable by the run's user under every profile, creating each with exclusive create and resolving every component with no symlink, and kills an exec whose output of the run exceeds disk_bytes (LIMIT_EXCEEDED). The host reads them, and computes every directory digest, with no symlink following and through a size-limited reader. Output is untrusted data: the host scans it for issued handles and canaries, and labels and delimits it before any prompt use.
// Payloads: requests and responses carry ids, paths, digests and counts, never file contents or output (a platform invariant: payloads carry references, never content).
// Status codes: INVALID_ARGUMENT for a request that breaks a bound; ALREADY_EXISTS for a Create or Exec whose key is taken by a different request; FAILED_PRECONDITION for a profile the host does not offer or MICROVM, an image or snapshot the provider does not hold or that fails its digest, a host path whose resolution, work root, live overlap or earlier use by a destroyed run the Paths rule refuses, a host tree holding a multi-link file or another user's entry at Create, a restore into non-empty directories or from a snapshot holding an entry at or under a masked path of the new spec, a Create of a destroyed run, and an Exec or Snapshot of a run that is not running; NOT_FOUND for a Wait of an unknown exec; RESOURCE_EXHAUSTED when the host has no capacity, a run has 16 execs running or 64 snapshots, or a snapshot exceeds disk_bytes or the per-host byte cap; UNAVAILABLE for a runtime failure the caller may retry, which creates nothing. A provider never reports an effect it did not make.
// Evidence: the host records the requested profile, image digest and limits, the provider's claims and its own checks. No RPC declares evidence yet; its record types are fixed with the sandbox context, and no spec tag ships while they are missing.
// Bounds: every string is at most 1024 bytes and every repeated field at most 256 entries unless stated; over-limit input is rejected, never truncated; duplicate entries in any repeated field are rejected; enums are closed, and zero and unknown numbers are rejected; a duration is never negative. Printable means Unicode categories L, M, N, P, S and U+0020. A rejection names an index-based field path and the violated bound, never the offending value.
// Every element is since 1.0.0.

// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: numinance/ports/sandboxprovider/v1/sandboxprovider.proto

package sandboxproviderv1connect

import (
	connect "connectrpc.com/connect"
	context "context"
	errors "errors"
	v1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/sandboxprovider/v1"
	http "net/http"
	strings "strings"
)

// This is a compile-time assertion to ensure that this generated file and the connect package are
// compatible. If you get a compiler error that this constant is not defined, this code was
// generated with a version of connect newer than the one compiled into your binary. You can fix the
// problem by either regenerating this code with an older version of connect or updating the connect
// version compiled into your binary.
const _ = connect.IsAtLeastVersion1_13_0

const (
	// SandboxProviderServiceName is the fully-qualified name of the SandboxProviderService service.
	SandboxProviderServiceName = "numinance.ports.sandboxprovider.v1.SandboxProviderService"
)

// These constants are the fully-qualified names of the RPCs defined in this package. They're
// exposed at runtime as Spec.Procedure and as the final two segments of the HTTP route.
//
// Note that these are different from the fully-qualified method names used by
// google.golang.org/protobuf/reflect/protoreflect. To convert from these constants to
// reflection-formatted method names, remove the leading slash and convert the remaining slash to a
// period.
const (
	// SandboxProviderServiceCreateProcedure is the fully-qualified name of the SandboxProviderService's
	// Create RPC.
	SandboxProviderServiceCreateProcedure = "/numinance.ports.sandboxprovider.v1.SandboxProviderService/Create"
	// SandboxProviderServiceExecProcedure is the fully-qualified name of the SandboxProviderService's
	// Exec RPC.
	SandboxProviderServiceExecProcedure = "/numinance.ports.sandboxprovider.v1.SandboxProviderService/Exec"
	// SandboxProviderServiceWaitProcedure is the fully-qualified name of the SandboxProviderService's
	// Wait RPC.
	SandboxProviderServiceWaitProcedure = "/numinance.ports.sandboxprovider.v1.SandboxProviderService/Wait"
	// SandboxProviderServiceSnapshotProcedure is the fully-qualified name of the
	// SandboxProviderService's Snapshot RPC.
	SandboxProviderServiceSnapshotProcedure = "/numinance.ports.sandboxprovider.v1.SandboxProviderService/Snapshot"
	// SandboxProviderServiceDestroyProcedure is the fully-qualified name of the
	// SandboxProviderService's Destroy RPC.
	SandboxProviderServiceDestroyProcedure = "/numinance.ports.sandboxprovider.v1.SandboxProviderService/Destroy"
)

// SandboxProviderServiceClient is a client for the
// numinance.ports.sandboxprovider.v1.SandboxProviderService service.
type SandboxProviderServiceClient interface {
	// Create creates the sandbox of spec.run_id and returns when it is ready to Exec. One sandbox per run_id: the provider keeps the key of a created sandbox, across its own restarts, at least until the spec's deadline, and looks it up before any other check but the bounds; a Create with the same spec is a replay that answers the first success and creates nothing, another spec is ALREADY_EXISTS, and a Create of a destroyed run is FAILED_PRECONDITION. LOW: nothing runs until Exec, and Destroy releases all of it; the directories are the host's. Dry run checks the spec and whether the provider can create it on this host (profile, image, paths, capacity), creates nothing, and answers what it would create.
	Create(context.Context, *connect.Request[v1.CreateRequest]) (*connect.Response[v1.CreateResponse], error)
	// Exec starts one command in the sandbox of run_id and returns once it has started; Wait is the get of the operation, a separate READ invocation. There is no cancel: the exec timeout, Destroy and the sandbox deadline end it. On the plugin protocol Exec is one terminal Invoke message, whose success means started. MEDIUM: the command is untrusted code; under CONTAINER and GVISOR it is confined to the disposable sandbox, and its effects outside pass the egress proxy and the broker under the run's grant. The decision for an Exec takes the higher of MEDIUM and the risk of every granted credential intent (from the broker catalog) and egress destination (from the signed network policy), and an unclassified one counts as PRIVILEGED. An Exec with the same (run_id, exec_seq) and the same request is a replay that starts nothing; another request is ALREADY_EXISTS. The provider looks the key up before any check but the bounds that depend on no time or state, so the timeout against the time left and the run's state are checked only for a new key, and a replay near the deadline or after Destroy is still answered. No dry run: running the command is the only way to learn what it does, and the sandbox is discarded unless the host takes a candidate from the worktree.
	Exec(context.Context, *connect.Request[v1.ExecRequest]) (*connect.Response[v1.ExecResponse], error)
	// Wait returns the state of one exec when it ends or when wait_timeout passes, whichever is first; a RUNNING answer means call again.
	Wait(context.Context, *connect.Request[v1.WaitRequest]) (*connect.Response[v1.WaitResponse], error)
	// Snapshot freezes every process of run_id (under PROCESS with a cgroup freezer where one exists, otherwise by stopping every process of the run's user, repeated until two passes find no running one, and continuing them after; a provider that cannot freeze refuses Snapshot with FAILED_PRECONDITION), takes a content-addressed copy of the worktree and artifact directories, computes their tree digests on the copy, and thaws the processes before it returns. It holds a reference (run_id, snapshot) to the copy in the provider's storage on the runner's host, outside the run's disk_bytes. The copy leaves out every entry at or under the spec's masked_paths (compared under NFC and case folding), records of each file only its content and owner execute bit (no other mode bit, no owner), records a symlink as a link and never reads through it, refuses with FAILED_PRECONDITION a file with more than one hard link, a special file (FIFO, socket, device), an entry whose name breaks the path rules (printable NFC segments) and two names equal after NFC normalization or after NFC normalization and case folding, and refuses with RESOURCE_EXHAUSTED a copy whose apparent sizes exceed disk_bytes. Content is deleted when its last reference is released: by Destroy with release_snapshots, or by the provider between 7 and 30 days after the run's deadline, after which a later run starts fresh. At most 64 snapshots per run, within a per-host byte cap the provider is configured with. LOW: it adds a copy and changes nothing in the sandbox. Not idempotent: a retry takes a new copy of the current state; equal content has an equal digest, so a retry is harmless. No dry run: there is no plan to preview, and the copy is the only effect.
	Snapshot(context.Context, *connect.Request[v1.SnapshotRequest]) (*connect.Response[v1.SnapshotResponse], error)
	// Destroy kills every process of run_id, removes the sandbox and its tmpfs and releases its resources; the host directories stay. Once no process of the run's user remains, Destroy hands the worktree and artifact roots and every entry under them to the host's user without following any symlink, with modes normalized as at restore (no setuid, setgid or sticky bit, no group or other write). The run's uid is the host uid its processes ran as (under a user namespace, the host uid the run's user maps to). It changes in place an entry owned by the run's uid and, under CONTAINER and GVISOR only, one owned by the host's user (base content there; under PROCESS, where Create hands every entry to the run's user, a host-user entry is foreign). It removes, never changing its owner, mode or content, every other entry (a directory with everything under it, never following links) and every non-directory with more than one link, judged on the link counts before any removal, so a run that hardlinks a file loses every name of it; the link-count test applies to non-directories only. It reports each removal to the host, which records an abnormal action (the report's channel is owed by the sandbox context). At Create and at Destroy the handover also removes every POSIX ACL and every user.*, trusted.* and security.capability extended attribute, and clears every file flag; at Create it gives the run's entries an LSM label unique to the run (a per-run MCS pair, never a shared container label), and at Destroy it resets every LSM label to the work root's default context. The Snapshot copy, the handovers, the removals and the host copy never cross a filesystem boundary (every entry's st_dev equals its root's) and refuse a mount point inside a tree; mount points are found from the mount table or mount ids (statx STATX_MNT_ID, /proc/self/mountinfo), never by a stat that reaches a mounted file system. Under PROCESS, Snapshot freezes first, then detaches every mount under the run's directories (lazy unmount, and abort of every FUSE connection owned by the run's uid), then copies, then thaws; the Destroy handover detaches them after every process is dead. A run counts as destroyed only once the handover is complete: until then every Destroy (a retry of one that failed partway or was cut by a provider restart, or the host's next Destroy after the deadline destroy) completes it and answers OK only after the handover; one that cannot is UNAVAILABLE and the caller retries. The host does not act on that OK: it then copies the kept tree into its own storage, opening each entry without following links (an entry that is not a regular file, directory or symlink is refused unopened), verifying owner, link count, type and mode with fstat on the opened descriptor, and reading content from that descriptor through a reader bounded by disk_bytes of apparent size, within the name rules of Snapshot and an entry bound (1,000,000 by default, at most 10,000,000) and depth bound (at most 256), a configuration above a ceiling being refused, and copying no ACL, extended attribute or file flag; under CONTAINER and GVISOR it accepts a host-owned entry only at its own base path with that path's base digest and refuses every other (a rename or copy by the run is run-owned on disk); any refusal is an abnormal action, and the host builds, checks and uses the candidate only from that copy. It deletes the kept directories once the copy is taken or the run fails, after keeping a refused tree for the incident workflow in an access-restricted quarantine store under bounded retention, which evidence references by digest only (never in an exported evidence bundle), scanned and redacted for handles before any reviewer view. Destroy of a destroyed or unknown run succeeds, and still releases the run's snapshot references when release_snapshots is set. Each exec's key and final state survive Destroy until the spec's deadline, like the Create key, so Wait answers KILLED for an exec Destroy ended. LOW: under every profile, nothing the host keeps is removed, untouched base content included, but names the run had no right to keep (foreign entries, multi-link files), and snapshot content another reference holds stays. No dry run: the release is the whole effect, and the host already knows what the run holds.
	Destroy(context.Context, *connect.Request[v1.DestroyRequest]) (*connect.Response[v1.DestroyResponse], error)
}

// NewSandboxProviderServiceClient constructs a client for the
// numinance.ports.sandboxprovider.v1.SandboxProviderService service. By default, it uses the
// Connect protocol with the binary Protobuf Codec, asks for gzipped responses, and sends
// uncompressed requests. To use the gRPC or gRPC-Web protocols, supply the connect.WithGRPC() or
// connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewSandboxProviderServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) SandboxProviderServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	sandboxProviderServiceMethods := v1.File_numinance_ports_sandboxprovider_v1_sandboxprovider_proto.Services().ByName("SandboxProviderService").Methods()
	return &sandboxProviderServiceClient{
		create: connect.NewClient[v1.CreateRequest, v1.CreateResponse](
			httpClient,
			baseURL+SandboxProviderServiceCreateProcedure,
			connect.WithSchema(sandboxProviderServiceMethods.ByName("Create")),
			connect.WithClientOptions(opts...),
		),
		exec: connect.NewClient[v1.ExecRequest, v1.ExecResponse](
			httpClient,
			baseURL+SandboxProviderServiceExecProcedure,
			connect.WithSchema(sandboxProviderServiceMethods.ByName("Exec")),
			connect.WithClientOptions(opts...),
		),
		wait: connect.NewClient[v1.WaitRequest, v1.WaitResponse](
			httpClient,
			baseURL+SandboxProviderServiceWaitProcedure,
			connect.WithSchema(sandboxProviderServiceMethods.ByName("Wait")),
			connect.WithClientOptions(opts...),
		),
		snapshot: connect.NewClient[v1.SnapshotRequest, v1.SnapshotResponse](
			httpClient,
			baseURL+SandboxProviderServiceSnapshotProcedure,
			connect.WithSchema(sandboxProviderServiceMethods.ByName("Snapshot")),
			connect.WithClientOptions(opts...),
		),
		destroy: connect.NewClient[v1.DestroyRequest, v1.DestroyResponse](
			httpClient,
			baseURL+SandboxProviderServiceDestroyProcedure,
			connect.WithSchema(sandboxProviderServiceMethods.ByName("Destroy")),
			connect.WithClientOptions(opts...),
		),
	}
}

// sandboxProviderServiceClient implements SandboxProviderServiceClient.
type sandboxProviderServiceClient struct {
	create   *connect.Client[v1.CreateRequest, v1.CreateResponse]
	exec     *connect.Client[v1.ExecRequest, v1.ExecResponse]
	wait     *connect.Client[v1.WaitRequest, v1.WaitResponse]
	snapshot *connect.Client[v1.SnapshotRequest, v1.SnapshotResponse]
	destroy  *connect.Client[v1.DestroyRequest, v1.DestroyResponse]
}

// Create calls numinance.ports.sandboxprovider.v1.SandboxProviderService.Create.
func (c *sandboxProviderServiceClient) Create(ctx context.Context, req *connect.Request[v1.CreateRequest]) (*connect.Response[v1.CreateResponse], error) {
	return c.create.CallUnary(ctx, req)
}

// Exec calls numinance.ports.sandboxprovider.v1.SandboxProviderService.Exec.
func (c *sandboxProviderServiceClient) Exec(ctx context.Context, req *connect.Request[v1.ExecRequest]) (*connect.Response[v1.ExecResponse], error) {
	return c.exec.CallUnary(ctx, req)
}

// Wait calls numinance.ports.sandboxprovider.v1.SandboxProviderService.Wait.
func (c *sandboxProviderServiceClient) Wait(ctx context.Context, req *connect.Request[v1.WaitRequest]) (*connect.Response[v1.WaitResponse], error) {
	return c.wait.CallUnary(ctx, req)
}

// Snapshot calls numinance.ports.sandboxprovider.v1.SandboxProviderService.Snapshot.
func (c *sandboxProviderServiceClient) Snapshot(ctx context.Context, req *connect.Request[v1.SnapshotRequest]) (*connect.Response[v1.SnapshotResponse], error) {
	return c.snapshot.CallUnary(ctx, req)
}

// Destroy calls numinance.ports.sandboxprovider.v1.SandboxProviderService.Destroy.
func (c *sandboxProviderServiceClient) Destroy(ctx context.Context, req *connect.Request[v1.DestroyRequest]) (*connect.Response[v1.DestroyResponse], error) {
	return c.destroy.CallUnary(ctx, req)
}

// SandboxProviderServiceHandler is an implementation of the
// numinance.ports.sandboxprovider.v1.SandboxProviderService service.
type SandboxProviderServiceHandler interface {
	// Create creates the sandbox of spec.run_id and returns when it is ready to Exec. One sandbox per run_id: the provider keeps the key of a created sandbox, across its own restarts, at least until the spec's deadline, and looks it up before any other check but the bounds; a Create with the same spec is a replay that answers the first success and creates nothing, another spec is ALREADY_EXISTS, and a Create of a destroyed run is FAILED_PRECONDITION. LOW: nothing runs until Exec, and Destroy releases all of it; the directories are the host's. Dry run checks the spec and whether the provider can create it on this host (profile, image, paths, capacity), creates nothing, and answers what it would create.
	Create(context.Context, *connect.Request[v1.CreateRequest]) (*connect.Response[v1.CreateResponse], error)
	// Exec starts one command in the sandbox of run_id and returns once it has started; Wait is the get of the operation, a separate READ invocation. There is no cancel: the exec timeout, Destroy and the sandbox deadline end it. On the plugin protocol Exec is one terminal Invoke message, whose success means started. MEDIUM: the command is untrusted code; under CONTAINER and GVISOR it is confined to the disposable sandbox, and its effects outside pass the egress proxy and the broker under the run's grant. The decision for an Exec takes the higher of MEDIUM and the risk of every granted credential intent (from the broker catalog) and egress destination (from the signed network policy), and an unclassified one counts as PRIVILEGED. An Exec with the same (run_id, exec_seq) and the same request is a replay that starts nothing; another request is ALREADY_EXISTS. The provider looks the key up before any check but the bounds that depend on no time or state, so the timeout against the time left and the run's state are checked only for a new key, and a replay near the deadline or after Destroy is still answered. No dry run: running the command is the only way to learn what it does, and the sandbox is discarded unless the host takes a candidate from the worktree.
	Exec(context.Context, *connect.Request[v1.ExecRequest]) (*connect.Response[v1.ExecResponse], error)
	// Wait returns the state of one exec when it ends or when wait_timeout passes, whichever is first; a RUNNING answer means call again.
	Wait(context.Context, *connect.Request[v1.WaitRequest]) (*connect.Response[v1.WaitResponse], error)
	// Snapshot freezes every process of run_id (under PROCESS with a cgroup freezer where one exists, otherwise by stopping every process of the run's user, repeated until two passes find no running one, and continuing them after; a provider that cannot freeze refuses Snapshot with FAILED_PRECONDITION), takes a content-addressed copy of the worktree and artifact directories, computes their tree digests on the copy, and thaws the processes before it returns. It holds a reference (run_id, snapshot) to the copy in the provider's storage on the runner's host, outside the run's disk_bytes. The copy leaves out every entry at or under the spec's masked_paths (compared under NFC and case folding), records of each file only its content and owner execute bit (no other mode bit, no owner), records a symlink as a link and never reads through it, refuses with FAILED_PRECONDITION a file with more than one hard link, a special file (FIFO, socket, device), an entry whose name breaks the path rules (printable NFC segments) and two names equal after NFC normalization or after NFC normalization and case folding, and refuses with RESOURCE_EXHAUSTED a copy whose apparent sizes exceed disk_bytes. Content is deleted when its last reference is released: by Destroy with release_snapshots, or by the provider between 7 and 30 days after the run's deadline, after which a later run starts fresh. At most 64 snapshots per run, within a per-host byte cap the provider is configured with. LOW: it adds a copy and changes nothing in the sandbox. Not idempotent: a retry takes a new copy of the current state; equal content has an equal digest, so a retry is harmless. No dry run: there is no plan to preview, and the copy is the only effect.
	Snapshot(context.Context, *connect.Request[v1.SnapshotRequest]) (*connect.Response[v1.SnapshotResponse], error)
	// Destroy kills every process of run_id, removes the sandbox and its tmpfs and releases its resources; the host directories stay. Once no process of the run's user remains, Destroy hands the worktree and artifact roots and every entry under them to the host's user without following any symlink, with modes normalized as at restore (no setuid, setgid or sticky bit, no group or other write). The run's uid is the host uid its processes ran as (under a user namespace, the host uid the run's user maps to). It changes in place an entry owned by the run's uid and, under CONTAINER and GVISOR only, one owned by the host's user (base content there; under PROCESS, where Create hands every entry to the run's user, a host-user entry is foreign). It removes, never changing its owner, mode or content, every other entry (a directory with everything under it, never following links) and every non-directory with more than one link, judged on the link counts before any removal, so a run that hardlinks a file loses every name of it; the link-count test applies to non-directories only. It reports each removal to the host, which records an abnormal action (the report's channel is owed by the sandbox context). At Create and at Destroy the handover also removes every POSIX ACL and every user.*, trusted.* and security.capability extended attribute, and clears every file flag; at Create it gives the run's entries an LSM label unique to the run (a per-run MCS pair, never a shared container label), and at Destroy it resets every LSM label to the work root's default context. The Snapshot copy, the handovers, the removals and the host copy never cross a filesystem boundary (every entry's st_dev equals its root's) and refuse a mount point inside a tree; mount points are found from the mount table or mount ids (statx STATX_MNT_ID, /proc/self/mountinfo), never by a stat that reaches a mounted file system. Under PROCESS, Snapshot freezes first, then detaches every mount under the run's directories (lazy unmount, and abort of every FUSE connection owned by the run's uid), then copies, then thaws; the Destroy handover detaches them after every process is dead. A run counts as destroyed only once the handover is complete: until then every Destroy (a retry of one that failed partway or was cut by a provider restart, or the host's next Destroy after the deadline destroy) completes it and answers OK only after the handover; one that cannot is UNAVAILABLE and the caller retries. The host does not act on that OK: it then copies the kept tree into its own storage, opening each entry without following links (an entry that is not a regular file, directory or symlink is refused unopened), verifying owner, link count, type and mode with fstat on the opened descriptor, and reading content from that descriptor through a reader bounded by disk_bytes of apparent size, within the name rules of Snapshot and an entry bound (1,000,000 by default, at most 10,000,000) and depth bound (at most 256), a configuration above a ceiling being refused, and copying no ACL, extended attribute or file flag; under CONTAINER and GVISOR it accepts a host-owned entry only at its own base path with that path's base digest and refuses every other (a rename or copy by the run is run-owned on disk); any refusal is an abnormal action, and the host builds, checks and uses the candidate only from that copy. It deletes the kept directories once the copy is taken or the run fails, after keeping a refused tree for the incident workflow in an access-restricted quarantine store under bounded retention, which evidence references by digest only (never in an exported evidence bundle), scanned and redacted for handles before any reviewer view. Destroy of a destroyed or unknown run succeeds, and still releases the run's snapshot references when release_snapshots is set. Each exec's key and final state survive Destroy until the spec's deadline, like the Create key, so Wait answers KILLED for an exec Destroy ended. LOW: under every profile, nothing the host keeps is removed, untouched base content included, but names the run had no right to keep (foreign entries, multi-link files), and snapshot content another reference holds stays. No dry run: the release is the whole effect, and the host already knows what the run holds.
	Destroy(context.Context, *connect.Request[v1.DestroyRequest]) (*connect.Response[v1.DestroyResponse], error)
}

// NewSandboxProviderServiceHandler builds an HTTP handler from the service implementation. It
// returns the path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewSandboxProviderServiceHandler(svc SandboxProviderServiceHandler, opts ...connect.HandlerOption) (string, http.Handler) {
	sandboxProviderServiceMethods := v1.File_numinance_ports_sandboxprovider_v1_sandboxprovider_proto.Services().ByName("SandboxProviderService").Methods()
	sandboxProviderServiceCreateHandler := connect.NewUnaryHandler(
		SandboxProviderServiceCreateProcedure,
		svc.Create,
		connect.WithSchema(sandboxProviderServiceMethods.ByName("Create")),
		connect.WithHandlerOptions(opts...),
	)
	sandboxProviderServiceExecHandler := connect.NewUnaryHandler(
		SandboxProviderServiceExecProcedure,
		svc.Exec,
		connect.WithSchema(sandboxProviderServiceMethods.ByName("Exec")),
		connect.WithHandlerOptions(opts...),
	)
	sandboxProviderServiceWaitHandler := connect.NewUnaryHandler(
		SandboxProviderServiceWaitProcedure,
		svc.Wait,
		connect.WithSchema(sandboxProviderServiceMethods.ByName("Wait")),
		connect.WithHandlerOptions(opts...),
	)
	sandboxProviderServiceSnapshotHandler := connect.NewUnaryHandler(
		SandboxProviderServiceSnapshotProcedure,
		svc.Snapshot,
		connect.WithSchema(sandboxProviderServiceMethods.ByName("Snapshot")),
		connect.WithHandlerOptions(opts...),
	)
	sandboxProviderServiceDestroyHandler := connect.NewUnaryHandler(
		SandboxProviderServiceDestroyProcedure,
		svc.Destroy,
		connect.WithSchema(sandboxProviderServiceMethods.ByName("Destroy")),
		connect.WithHandlerOptions(opts...),
	)
	return "/numinance.ports.sandboxprovider.v1.SandboxProviderService/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case SandboxProviderServiceCreateProcedure:
			sandboxProviderServiceCreateHandler.ServeHTTP(w, r)
		case SandboxProviderServiceExecProcedure:
			sandboxProviderServiceExecHandler.ServeHTTP(w, r)
		case SandboxProviderServiceWaitProcedure:
			sandboxProviderServiceWaitHandler.ServeHTTP(w, r)
		case SandboxProviderServiceSnapshotProcedure:
			sandboxProviderServiceSnapshotHandler.ServeHTTP(w, r)
		case SandboxProviderServiceDestroyProcedure:
			sandboxProviderServiceDestroyHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedSandboxProviderServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedSandboxProviderServiceHandler struct{}

func (UnimplementedSandboxProviderServiceHandler) Create(context.Context, *connect.Request[v1.CreateRequest]) (*connect.Response[v1.CreateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.sandboxprovider.v1.SandboxProviderService.Create is not implemented"))
}

func (UnimplementedSandboxProviderServiceHandler) Exec(context.Context, *connect.Request[v1.ExecRequest]) (*connect.Response[v1.ExecResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.sandboxprovider.v1.SandboxProviderService.Exec is not implemented"))
}

func (UnimplementedSandboxProviderServiceHandler) Wait(context.Context, *connect.Request[v1.WaitRequest]) (*connect.Response[v1.WaitResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.sandboxprovider.v1.SandboxProviderService.Wait is not implemented"))
}

func (UnimplementedSandboxProviderServiceHandler) Snapshot(context.Context, *connect.Request[v1.SnapshotRequest]) (*connect.Response[v1.SnapshotResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.sandboxprovider.v1.SandboxProviderService.Snapshot is not implemented"))
}

func (UnimplementedSandboxProviderServiceHandler) Destroy(context.Context, *connect.Request[v1.DestroyRequest]) (*connect.Response[v1.DestroyResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("numinance.ports.sandboxprovider.v1.SandboxProviderService.Destroy is not implemented"))
}
