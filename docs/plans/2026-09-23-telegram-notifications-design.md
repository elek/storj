# Telegram notifications for confirmed nodes — design

Date: 2026-09-23

Adopts the Telegram notifications of [spiridon](https://github.com/elek/spiridon)
(`bot/`): node operators get the status changes of their nodes in a Telegram
chat, not only by email.

## Scope

- Only **confirmed** nodes are notified: nodes with an `owner` tag signed by
  this satellite (see `2026-09-18-console-nodes-submenu-design.md`).
- The events are the existing node events (`satellite/nodeevents`): offline,
  back online, disqualified, suspended / unsuspended, below minimum version.
  No new event detection.
- The console gets a **Notifications** menu item, right below **Nodes**.

## Connecting a chat

Spiridon subscribes with `/subscribe <nodeid>`, which anyone can send for any
node. Here the chat is tied to the console account instead:

1. The Notifications page asks for a link: `https://t.me/<bot>?start=<token>`.
2. The token is the user ID and an expiration, authenticated with an HMAC keyed
   by the bot token (40 bytes, 54 characters of URL base64; Telegram allows 64).
   Nothing is stored for it.
3. Opening the link in Telegram and pressing Start sends `/start <token>` to the
   bot, which verifies it and records the chat.

The bot (`telegram.Bot`) long-polls `getUpdates`. Telegram allows one poller per
token, so it runs in exactly one process: `--components=...,telegram.Bot`.

## Storage: node tags, no migration

The chat is recorded as a `telegram` node tag signed by the satellite, on each
confirmed node of the user: value = user UUID (16 bytes) + chat ID (int64). A
new table would need a numbered migration, which conflicts on every rebase of
this branch.

- Lookup is per *user*, not per node: the most recent `telegram` tag naming the
  user among the nodes of the user's email. Nodes confirmed after connecting are
  covered without rewriting anything.
- The user ID in the value means a tag left on a node that changed owners is
  never used for the new owner.
- Disconnecting overwrites every tag of the user with a tombstone (user ID only).

## Choosing the notifier with mud

`nodeevents.Notifier` stays an interface with one active implementation
(`RegisterInterfaceImplementation`), email (customer.io) by default:

| mode             | `--components`                                     |
|------------------|----------------------------------------------------|
| email (default)  | —                                                  |
| Telegram only    | `nodeevents.Notifier=telegram.Notifier`            |
| email + Telegram | `nodeevents.Notifier=nodeevents.MultiNotifier`     |

`MultiNotifier` fans out to the `[]nodeevents.Notifier` multibinding, which each
implementation joins with `mud.Implementation`. A future channel (ntfy, ...)
joins the same list without touching `nodeevents`.

The chore only runs with `--node-events.send-node-emails`, which also gates the
insertion of the events, whatever the notifier.

Telegram sending is best-effort: a failed chat is logged, not returned, because
the chore retries a failed batch with every notifier and would duplicate the
emails.
