# Policy editions and consent records

The [public policy site](https://policies.taskly.iy0306.com/) is maintained in
[`taskly-policy-web`](https://github.com/asari194617/taskly-policy-web). It
publishes versioned policy releases from
[`taskly-policy`](https://github.com/asari194617/taskly-policy). The public
policy set released as [`v2026-11-16.1`](https://github.com/asari194617/taskly-policy/releases/tag/v2026-11-16.1)
took effect on 2026-11-16. Taskly's in-app consent still uses the fixed
`2026-09-22.1` edition defined in
[`consent.go`](../../apps/server/internal/legal/consent.go) and
[`ConsentPage.tsx`](../../apps/web/src/react/pages/ConsentPage.tsx).

## Initial catalogued edition

The three then-public Notion documents were snapshotted on 2026-09-22 as
consent edition `2026-09-22.1`. The archived texts are:

- [Terms](terms-2026-09-22.1.md)
- [Privacy policy](privacy-2026-09-22.1.md)
- [Security policy](security-2026-09-22.1.md)

SHA-256 of the archived Markdown (in the order above):

```text
terms     688178ab0fac56e973747d16168313c2bb7b998e6a3fd9c3527d46fa72f4046d
privacy   05d950de9df6e88fea9810d6d4c3e5d61167d963fa5e68a1c8fc8c0fad6ae46a
security  2bcb886a3902897d5fbf57ff0fd2de1c3346fc4172cc85974e75b5d2727f3b5e
```

The [2026-09-23 privacy revision](privacy-policy.md) (PR #418) is a historical
reference. The consent archive preserves the text shown when users accepted
`2026-09-22.1`.

The web consent page links to immutable paths at
`https://policies.taskly.iy0306.com/{terms,privacy,security}/2026-09-22.1/`.
The archived edition is the evidence of what was displayed for an event.
Publishing a new document on the policy site alone must never change the
archived edition or the server's accepted version.

## Migration and access

The `policy_consents` definition in `apps/server/schema.sql` adds append-only
events with user ID, acceptance time, all three document versions and the display language. The legacy
`users.is_agreed_policies` value is retained for old API compatibility but
cannot establish when or which text a user accepted. No synthetic events are
created. Every existing user, including one with that flag set, must explicitly
accept the catalogued edition once. The first protected web load redirects to
`/consent`; authenticated REST, GraphQL and iCal access is denied until the
current edition is recorded. Account inspection, consent, logout and deletion
remain available before acceptance.

`accepted_at` is a MySQL `DATETIME(6)`; the API's MySQL connection uses
`Asia/Tokyo` when writing and reading it. Exports must label it JST and convert
to UTC explicitly when needed.

The consent event and compatibility flag update share a transaction. A stale
version or invalid display language is rejected. The server decides whether the
latest event is current. The client never supplies a timestamp or user ID.

## Publishing a revision

1. Follow the [policy repository's release process](https://github.com/asari194617/taskly-policy#改訂の手順)
   to freeze the new text and publish the release manifest with an effective
   date and change history. Preserve older files and URLs.
2. Verify that the new immutable policy URLs serve the released texts. If the
   revision requires new in-app consent, archive the same texts in
   `docs/legal/*-VERSION.md` and coordinate the following steps with the policy release.
3. Set `Version`, `PreviousVersion`, and a **future** `EffectiveAt` in
   `apps/server/internal/legal/consent.go` and update the consent page's `policyVersion`.
   Publish a change summary and effective date in the user announcement.
4. Deploy these changes sufficiently before the effective date to show the
   in-app notice on protected pages. Send the same notice through the existing
   email announcement process, recording the audience and send time in the
   release record. For the terms, this must happen before effectiveness under
   article 11. A release is not ready if that notification has not been sent.
5. Test an existing user before and after `EffectiveAt`: the prior version
   remains usable before that instant; after it, protected APIs return 403
   until a new event is recorded. A user may accept the new version early.

Changing `Version` makes reconsent mandatory for all three documents. If a
future change needs notice without new consent, that should be a deliberate
policy decision and code change reviewed separately; it must not be inferred
from the legacy boolean.

## Retention and access

Keep consent events for five years after account closure, unless a legal hold
requires longer retention. The user row is soft-deleted while events are
retained. After that period, delete events as part of the account erasure
process before hard-deleting the user. Document any deletion in the erasure
audit record.

Only operators with the `admin` role and a documented support or legal need
may request an event export. The application exposes no list or mutation for
these events; managers and ordinary users cannot query them through GraphQL or
REST. Production database read access and exports are controlled by the
operations team, with access and purpose recorded in the incident/support
ticket. Exports must include the archive revision and be removed when that
request is closed.

## Deployment and rollback

Apply `apps/server/schema.sql` with Atlas before deploying an API that uses
consent events; see [schema operations](../initial-settings.md#dbスキーマ管理).
Historical migration numbers are not deployment commands. An older API that
predates consent events ignores the table and can continue reading and writing
the legacy boolean; it will not create consent events. Rolling back only the API image leaves events intact.
If the migration itself must be undone, stop consent writes, take a verified
backup, and use a reviewed corrective migration. Do not drop the event table
to roll back the feature.
