# Policy editions and consent records

## Initial catalogued edition

The three public Notion documents were snapshotted on 2026-09-22 as edition
`2026-09-22.1`. Their public pages now show that edition, the effective date,
and a change history. The archived texts are:

- [Terms](terms-2026-09-22.1.md)
- [Privacy policy](privacy-2026-09-22.1.md)
- [Security policy](security-2026-09-22.1.md)

SHA-256 of the archived Markdown (in the order above):

```text
terms     688178ab0fac56e973747d16168313c2bb7b998e6a3fd9c3527d46fa72f4046d
privacy   05d950de9df6e88fea9810d6d4c3e5d61167d963fa5e68a1c8fc8c0fad6ae46a
security  2bcb886a3902897d5fbf57ff0fd2de1c3346fc4172cc85974e75b5d2727f3b5e
```

The web consent page serves immutable paths built from these files. The archived
edition is the evidence of what was displayed for an event. Notion remains the
public editing location, but changing its text alone must never change the
archived edition or the server's accepted version.

## Migration and access

`0030_policy_consents.sql` adds append-only events with user ID, acceptance
time, all three document versions and the display language. The legacy
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

1. Freeze the exact revised text in new `docs/legal/*-VERSION.md` archives.
   Create matching versioned web pages. Preserve all older files and URLs.
2. Update the three public Notion pages with version, effective date and a
   change entry. Confirm the public text matches the new archive.
3. Set `Version`, `PreviousVersion`, and a **future** `EffectiveAt` in
   `internal/legal/consent.go`; update the consent page's `policyVersion`.
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

Run migration 0030 before deploying the new API. The old API image ignores the
new table and can continue reading and writing the legacy boolean; it will not
create consent events. Rolling back only the API image leaves events intact.
If the migration itself must be undone, stop consent writes, take a verified
backup, and use a reviewed corrective migration. Do not drop the event table
to roll back the feature.
