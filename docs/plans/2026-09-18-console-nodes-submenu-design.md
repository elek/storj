# Console "Nodes" submenu — design

Date: 2026-09-18
Branch: `console-nodes-submenu` (long-lived feature branch)

## Goal

Give satellite console users a "Nodes" page listing the storage nodes whose
operator email matches their own account email.

This satellite primarily serves storage node operators, so the page is always
visible rather than gated behind a config flag or a node-count check.

## Primary constraint

The change must survive repeated rebases onto upstream `main`. Every design
decision below trades code volume for a *small, stable* upstream diff. New code
lives in dedicated subdirectories; upstream files get a fixed extension hook
instead of feature-specific lines.

## Backend

### Extension hook

New package `satellite/console/consoleext`:

```go
type Deps struct {
    WithAuth func(http.Handler) http.Handler
}

type Extension interface {
    Name() string
    Register(router *mux.Router, deps Deps)
}

func Module(ball *mud.Ball) {
    mud.RegisterImplementation[[]Extension](ball)
}
```

`Deps` carries only what mud cannot inject — the console server's unexported
auth middleware. Real dependencies (`overlay.DB`, loggers, config) arrive
through each extension's own mud-provided constructor. `Deps` is a struct, not
a positional parameter list, so adding a dependency later is a one-field change
that does not break existing extensions.

Extensions self-register using mud's multibinding primitive, the same pattern
`rangedloop.Observer` uses (`satellite/nodeaudit/mud.go:21`):

```go
func Module(ball *mud.Ball) {
    mud.Provide[*Extension](ball, New)
    mud.Implementation[[]consoleext.Extension, *Extension](ball)
}
```

Upstream diff:

1. `consoleweb/server.go` — `NewServer` gains a trailing
   `extensions []consoleext.Extension`; `initializeRouter` gains a three-line
   registration loop.
2. `satellite/mud.go` — two `Module(ball)` calls, and `CreateConsoleServer`
   gains the same parameter, which mud fills automatically.
3. `satellite/api.go` and `satellite/console-api.go` pass `nil`. Only the
   modular (mud) path is supported; the classic peers are out of scope.

Adding a second submenu later costs **one line** in `satellite/mud.go`.

### Nodes endpoint

`GET /api/v0/nodes`, wrapped in `deps.WithAuth`.

1. `console.GetUser(ctx)` — 401 if absent.
2. `user.Status != console.Active` → 403. Only verified account emails may
   resolve to nodes.
3. `overlayDB.GetNodesByEmailInsensitive(ctx, user.Email, limit)`.

Response is the full list in one payload; the UI paginates client-side. The
dbx cursor type `Paged_Node_By_Email_Continuation` has only unexported fields,
so it cannot be serialized into a REST response without an upstream dbx change.
A typical operator has a handful of nodes, so a single capped response is
simpler and cheaper than inventing a cursor encoding.

DTO fields, all from `overlay.NodeDossier` with no extra queries: `id`,
`address`, `lastIPPort`, `wallet`, `walletFeatures`, `pieceCount`,
`lastContactSuccess`, `lastContactFailure`, `online`, `vettedAt`,
`disqualified`, `disqualificationReason`, `exitFinishedAt`, `countryCode`,
`version`, `freeDisk`, `createdAt`, `confirmed`.

`online` is computed server-side as `time.Since(lastContactSuccess) < 4h`,
matching `satellite/admin/nodes.go:75`. Client clocks are unreliable and the
two views should not disagree.

### Ownership confirmation

`POST /api/v0/nodes/{id}/confirm`, wrapped in the same `deps.WithAuth`.

1. `console.GetUser(ctx)` — 401 if absent, 403 if not `Active`.
2. Parse `{id}` as a `storj.NodeID` — 400 if malformed.
3. `overlayDB.Get(ctx, id)` — 404 if unknown.
4. `strings.EqualFold(dossier.Operator.Email, user.Email)` — 403 otherwise.
   Confirming re-runs the listing's own match rather than trusting that the
   node came off a list the user was shown earlier.
5. `overlayDB.UpdateNodeTags` writes one tag: `owner = user.ID.Bytes()`,
   `signer = <this satellite's NodeID>`, `signed_at = now`.

The GET response carries `confirmed`, true when the dossier has an `owner` tag
signed by this satellite whose value equals the requesting user's ID. A tag
naming somebody else reads as unconfirmed, so an operator who took over a
node's email can claim it; the row is keyed by `(node_id, name, signer)`, so
confirming overwrites the previous owner.

Both endpoints need only methods already on `overlay.DB` — `Get` and
`UpdateNodeTags` — so ownership costs the branch no new upstream interface
lines at all.

## Decisions and their rationale

### Email verification (console side only)

`node.email` is self-reported by the operator at check-in and is never
verified. The gate is therefore on the *console* user: `user.Status == Active`,
meaning they activated their account by email.

This does not prevent an operator from typing a stranger's email into their own
node config and having that node appear in the stranger's list. The exposure is
limited to that operator's own node's operational stats, so a node-side
ownership-proof flow was judged not worth the complexity.

### The owner tag is signed by identity, not by bytes

`node_tags` has no signature column. `contact.Service.processNodeTags`
(`satellite/contact/service.go:194`) verifies the signature on an incoming
`pb.SignedNodeTagSets` and then persists only the *signer's* NodeID alongside
the name and value. The signer column is therefore the whole attestation.

A satellite writing the row itself with `signer = <its own NodeID>` produces
exactly the row a satellite-signed tag set would have produced, and a node
cannot forge it: to get that signer into the table over the wire it would have
to present a signature that verifies against the satellite's key. Computing a
signature here and then discarding it would be ceremony, so the endpoint
writes the tag directly and documents why.

The consequence to accept: the confirmation cannot be exported. If the row is
lost, the user re-clicks Confirm. Handing the operator a signed
`pb.SignedNodeTagSet` to paste into their node config would survive that, at
the cost of a manual step per node, and was judged not worth it.

### Confirmation is an email claim, not a proof

`node.email` is still self-reported. Confirmation records that the console user
who controls that address says the node is theirs — it does not prove they run
it, and a `confirmed` node is exactly as trustworthy as the email match the
listing already relies on. Treat the tag as "this user claims this node",
signed by the satellite, not as verified ownership.

### Tags loaded with the dossier

`confirmed` needs one tag per listed node. `overlay.DB.GetNodeTags` is
single-node, so rendering a capped page would cost up to 1000 queries. Instead
`GetNodesByEmailInsensitive` aggregates `node_tags` into JSON in the same
statement — the pattern `SelectedNodes` already uses
(`satellite/satellitedb/overlaycache.go:454`) — and populates the existing
`NodeDossier.Tags` field. No interface change, one query.

The JSON decoding is duplicated rather than factored out of
`scanSelectedNodeWithTags`, because sharing it would mean editing upstream
code; on this branch an additive helper is cheaper than a rebase conflict.

### Audit count dropped

`total_audit_count` lives in the `reputations` table, and `reputation.DB`
exposes only single-node `Get` (`satellite/reputation/service.go:22`). Showing
it would mean either an N+1 loop or a new bulk interface method. `vettedAt` is
already on `NodeDossier` and conveys the same "is this node trusted yet"
signal at zero cost, so it is shown instead.

### Case-insensitive matching

The console stores `normalized_email = strings.ToUpper(email)`
(`satellite/satellitedb/consoledb/users.go:1558`) and keeps `user.Email` as
typed. `nodes.email` is likewise as-typed. The existing
`Paged_Node_By_Email` is an exact, case-sensitive match, so an operator who
signed up as `Bob@x.com` but configured `bob@x.com` would see an empty page
with no error.

dbx cannot express `LOWER(email) = LOWER(?)` — no `.dbx` file in the repo uses
a function in a `where` clause — and `RawDB()` is test-only. So a new method is
added to `overlay.DB` with hand-written SQL:

- `satellite/overlay/service.go` — one interface line.
- `satellite/satellitedb/overlaycache.go` — the query, scanning into
  `dbx.Node` and reusing the existing `convertDBNode`.
- `satellite/overlay/mockdb.go` — stub.

All three are additive, which rebases acceptably.

### No index, no migration

`nodes.email` has no index, so this query is a sequential scan — as is the
existing `GetNodesByEmail` used by the admin API. An index would be correct and
would matter more as the network grows, but it requires a numbered migration in
`satellite/satellitedb/migrations`, and sequential migration numbers conflict
on *every* upstream rebase. That is the worst possible file for a long-lived
branch, so it is deliberately out of scope.

**This is a known, accepted cost.** If the page proves hot in production, the
index should be added at the point the branch merges upstream, not before.

## Frontend

New `web/satellite/src/extensions/index.ts`:

```ts
export interface ExtensionNavItem {
    title: string;
    to: string;
    icon: Component;
    order?: number;
}
export const extensionRoutes: RouteRecordRaw[];
export const extensionNavItems: ExtensionNavItem[];
```

Upstream diff, three files:

1. `router/index.ts` — spread `...extensionRoutes` into the `AccountLayout`
   route group, reusing `/account`'s `beforeEnter: setPathBeforeAccountPage` so
   the sidebar "Back" item works when `/nodes` is reached from inside a
   project.
2. `AccountNav.vue` — `v-for` over `extensionNavItems`, placed after the
   divider following Settings and before the Resources menu.
3. `ProjectNav.vue` — the identical block at the same anchor, so the item stays
   visible while a project is open.

Everything else lives in `web/satellite/src/extensions/nodes/`: the view, the
table component, the API client, and types. No Pinia store — a single
read-only fetch does not justify touching the store index.

The table follows `DomainsTableComponent.vue`, but as a plain client-side
`v-data-table` since the endpoint returns the full list.

## Testing

- Backend: testplanet tests covering a matching node, a case-mismatched node,
  a non-`Active` user receiving 403, and an unauthenticated request receiving
  401.
- Confirmation: 401/403/400/404 rejections, a node registered to a different
  email receiving 403, and the written tag's signer, name and value.
- `confirmed` is false for a tag naming another user and for a tag with the
  right value but the wrong signer.
- `overlaycache` test for `GetNodesByEmailInsensitive` across case variants and
  for the tags it now returns.

## Explicitly out of scope

- Any node-side proof that an operator owns the email they configured, and so
  any proof behind a confirmation beyond that email match.
- Un-confirming. The tag can be overwritten by whoever the node's email points
  at next; there is no "release this node" button.
- Exporting the confirmation as a signed `pb.SignedNodeTagSet`.
- An index on `nodes.email` and its migration.
- Audit counts.
- Cursor pagination through the REST API.
- The classic (non-mud) satellite peers.
