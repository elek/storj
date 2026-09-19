# Node operator invites chore

Invite storage node operators who run a meaningful number of nodes to create a
satellite console account, so they can use the console nodes view
(`satellite/console/consoleext/nodes`) which lists the nodes whose operator
email matches the account email.

Every 5 minutes the chore looks for operator emails with at least 20 currently
active nodes and no console account, issues a registration token for each, and
emails the signup link.

## Decisions

### Invite bookkeeping: overload `registration_tokens.partner`

The invitation state is stored in `registration_tokens` itself, using the
`partner` column to hold the (lowercased) operator email. No new table, no
schema change, no dbx regeneration.

Everything the chore needs is derivable from that one table:

| question              | query                                                      |
|-----------------------|------------------------------------------------------------|
| active invite?        | `partner = ? AND owner_id IS NULL AND expires_at > now()`   |
| how many attempts?    | `count(*) WHERE partner = ?`                                |
| operator registered?  | `owner_id IS NOT NULL`                                      |

`partner` normally holds a partner name, used at registration to notify that
partner's admin (`consoleapi/auth.go:954-966`, keyed off
`console.partner-admin-email-mapping`). An email address never matches the
mapping, so the lookup is a silent no-op. This is the one consumer of the field;
the admin API validates `partner` against the mapping on create
(`admin/users.go:1436`), so admin-created tokens can never collide with
chore-created ones.

The alternative — a `node_operator_invites` table with its own named migration
— was rejected as more machinery than the feature needs. Adding an `email`
column to `registration_tokens` is not an option: it is dbx-managed and
satellitedb compares the live schema against the dbx snapshot in CI.

### Token shape

| field            | value | why                                                      |
|------------------|-------|----------------------------------------------------------|
| `project_limit`  | 1     | one project                                              |
| `storage_limit`  | 0     |                                                          |
| `bandwidth_limit`| 0     |                                                          |
| `segment_limit`  | 0     | account is for viewing nodes, not storing data           |
| `user_kind`      | NFR   | paid privileges without a billing relationship           |
| `expires_at`     | +7d   | also defines "active token"                              |
| `partner`        | email | lowercased operator email, see above                     |

`CreateUser` applies these with `!= nil` checks (`console/service.go:1692-1701`),
so an explicit `0` is applied rather than falling back to free-tier defaults.

NFR over Paid: both carry `HasPaidPrivileges` and neither expires as a trial, but
`IsPaid()` drives billing-freeze processing (`accountfreeze/billingfreezechore.go:152`),
`paidTier` in API responses, and an admin-side expectation that paid users have an
`UpgradeTime` (`admin/users.go:813`) that these accounts never had. NFR is
documented as exactly this case (`console/users.go:287-289`). If the operator
later adds a payment method the normal flow promotes them to `PaidUser`.

## Selection

All three tables live in satellitedb, so one query per tick returns exactly the
operators to invite:

```sql
WITH pools AS (
    SELECT lower(email) AS operator_email, count(*) AS node_count
    FROM nodes
    WHERE last_contact_success > $1        -- now - active window
      AND disqualified IS NULL
      AND exit_finished_at IS NULL
      AND email <> ''
    GROUP BY lower(email)
    HAVING count(*) >= $2                  -- min nodes
)
SELECT p.operator_email, p.node_count FROM pools p
WHERE NOT EXISTS (
        SELECT 1 FROM users u WHERE u.normalized_email = upper(p.operator_email))
  AND NOT EXISTS (
        SELECT 1 FROM registration_tokens t WHERE t.partner = p.operator_email
          AND (t.owner_id IS NOT NULL OR t.expires_at IS NULL OR t.expires_at > $3))
  AND (SELECT count(*) FROM registration_tokens t2
        WHERE t2.partner = p.operator_email) < $4
ORDER BY p.node_count DESC
LIMIT $5
```

Notes:

- Grouping is case insensitive, matching how the nodes view looks nodes up
  (`GetNodesByEmailInsensitive`).
- `users.normalized_email` is `UPPER(email)` and indexed
  (`consoledb/users.go:1558`, `users_email_status_index`).
- The user check ignores status and tenant: any account with that address —
  unverified, deleted, or in another tenant — blocks the invite.
- A used token (`owner_id IS NOT NULL`) blocks forever; an expired unused token
  re-qualifies the operator, capped by the attempt count.
- `registration_tokens.partner` is unindexed, which is fine: the planner hashes
  the small table once for the anti-join.
- Biggest operators are invited first when the backlog exceeds the batch size.

## Components

New package `satellite/nodeinvites`, shaped like `satellite/projectlimitevents`:

```
satellite/nodeinvites/chore.go   Chore, Config, DB interface, Run/RunOnce/Close
satellite/nodeinvites/mail.go    NodeOperatorInviteEmail
satellite/nodeinvites/mud.go     Module(ball)
satellite/satellitedb/nodeinvites.go   the query above
web/satellite/static/emails/NodeOperatorInvite.{html,txt}
```

Config (`node-invites`): `interval=5m`, `min-nodes=20`, `active-window=24h`,
`token-ttl=168h`, `max-attempts=3`, `batch-size=10`.

There is no `Enabled` flag and no wiring into `satellite/core.go` — the chore is
registered in mud only and enabled with an explicit mud selector.

`satellite.DB` gains `NodeInvites() nodeinvites.DB`, wired with
`mud.View[DB, nodeinvites.DB](ball, DB.NodeInvites)`. `console/mud.go` gains
`mud.View[DB, RegistrationTokens]`.

## Per-candidate flow

1. `CreateWithLimits` with the token shape above. **Invariant:** `partner` is the
   lowercased email, because selection matches `t.partner = lower(nodes.email)`.
   Storing it in original case silently breaks the duplicate check and the
   operator is re-invited every tick.
2. Link: `<console external address>/signup?token=<secret>`, secret
   `url.QueryEscape`d (base64url padding produces `=`).
3. Send `NodeOperatorInviteEmail` synchronously via `SendRendered`, so failures
   are observable.

A new template rather than reusing `NewUserRegistrationLink`, whose copy ("Your
storage is ready") is wrong for a zero-storage account.

Create-then-send means a send failure leaves a token that consumes one of the
three attempts, and `RegistrationTokens` has no `Delete` to undo it. So the
chore aborts the remainder of the batch on the first send error; the next tick
picks up fresh candidates. A mail outage costs one attempt per tick, not one per
operator.

## Testing

The chore is mud-only, so `testplanet` cannot reach it; tests use `mudplanet`
(`satellite/audit/reporter_test.go` is the pattern), with `WithModule` to bind
`MailSender` to a capture sender and `WithConfig` to tune thresholds.
`RunOnce(ctx)` is exposed for deterministic ticks.

`satellite/satellitedb/nodeinvites_test.go` (satellitedbtest) covers the query:
19 vs 20 nodes; case-insensitive pooling; stale nodes excluded; disqualified and
exited excluded; empty email excluded; verified and unverified users block; active
token blocks; used token blocks forever; expired unused token re-qualifies;
attempt cap; ordering and limit.

`satellite/nodeinvites/chore_test.go` covers behaviour: exact token fields,
lowercased `partner`, link shape, one mail per candidate, batch cap, abort on
send error. Plus an end-to-end case: register through `console.Service.CreateUser`
with the issued secret and assert the user is NFR with one project and a zero
segment limit.

## Known limitations

- Nothing binds the token to the invited address. An operator who registers with
  a different email gets an empty nodes view, and the chore keeps inviting the
  original address until the attempt cap.
- First enable on a live satellite drains the backlog at 10 per 5 minutes
  (~2,880/day).
- No opt-out: an operator who ignores the mail receives three over ~21 days.
- Selection and token creation are not serialized, so two instances running the chore
  at once can invite the same operator twice. Enable it on one instance.
- The `partner` overload needs a comment at both definitions so it is not
  reused unknowingly.
