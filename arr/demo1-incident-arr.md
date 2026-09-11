# demo1 destructive-change after-action report

## TL;DR

- On 2026-08-24 a `terraform apply -auto-approve` hit demo1 instead of localhost,
  because the shell held the wrong environment's credentials. It deleted live
  data by reconciling the instance down to importer-generated config that never
  contained budgets, funding, or labels in the first place.
- `terraform import` on its own is safe. It only reads and writes local state.
  But `import` blocks inside a `terraform apply` import and then reconcile, and
  absent means delete.
- Scope: 102 budget records deleted in 39.6 seconds, 83 label sets cleared, 44
  association links dropped, 1,360 write requests over 58 minutes.
- The tool is the root cause. The importer writes
  `# NOTE: budget/funding not imported` into every project file, and an apply
  reads that absence as intent to delete.
- The audit trail missed 31% of it, including all 102 budget deletions.
  Rebuilding the sequence took CloudWatch and the session transcripts.
- Status: resolved. demo1 was restored from a pre-incident database backup. This
  is a record, not an open item.
- Still open in the tooling: the importer gap. It now documents the omission but
  still emits it.

---

**What happened.** On 2026-08-24 at 22:58:31Z, a `terraform apply -input=false
-auto-approve` ran against demo1 with 843 `import` blocks, through the old Kion
provider (0.3.34), over configuration the old importer script produced. That
configuration does not carry budgets, funding, or labels. The importer writes
`# NOTE: budget/funding not imported - add manually if needed` into every project
it emits. Terraform reconciled the live instance down to that configuration. In
the 81 seconds the command ran it issued 537 writes, among them 102
`DELETE /api/v3/budget/{id}` in 39.6 seconds, and it cleared 83 label sets.

Those budget deletions also answer the funding question. Kion derives a funding
source's *Projects* count and *Budgeted* total from budget records. With all 102
gone, every funding source reads *No Projects* / *$0.00 Budgeted* without any
funding endpoint being called, which is why the audit trail showed no funding
operation to blame.

**Resolved.** demo1 was restored from a database backup predating the incident.
This report is the record of what happened, not an open item.

---

## Sources

Three, and they disagree. Where they do: CloudWatch is authoritative for *what
was requested*, the audit trail for *what payload was sent*, and the session
transcripts for *why*.

| source | covers | limits |
|---|---|---|
| `audit-trail-48h.json` | 1,846 rows, user 128, 08-24T21:15Z → 08-26T21:15Z | carries payloads (`log_json`), but records only 940 of 1,360 writes |
| CloudWatch `kion-logs` (AWS `644692554044`, us-east-1, stream `i-0f9c7a9d6ea1e8434-kion-webapi-1`) | every HTTP request, all users | no request bodies; 7-day retention, floor 2026-08-19T22:30:39Z, so this evidence expires around 2026-08-31 |
| Claude Code session transcripts (`0e7c7169…`, project `terraform-coverage-test`) | the commands that issued the requests | the only source that explains cause |

Everything here that the audit trail does not contain, meaning the budget
deletions, the true start time, and 420 unaudited writes, comes from CloudWatch
alone.

## Bounds

| | |
|---|---|
| Actor | `bshutter@kion.io` (user id 128), app API key 501, created 20:51:53.712Z |
| Writes by user 128, 08-19T22:30Z → 08-24T20:51Z | 0. Nothing precedes this session inside retention |
| First mutation | 2026-08-24T22:38:30.586Z |
| Last mutation | 2026-08-24T23:36:24.047Z |
| Damage window | 57 min 53 s |
| Write operations in window | 1,360 (audit trail records 940) |
| After the window | 191 writes on 08-25/08-26, none destructive: read-shaped `POST`s plus the 08-26 label restore |

---

## Timeline

All times UTC, 2026-08-24. "Writes" counts HTTP non-GET requests by user 128.

| time | writes | what | source |
|---|---:|---|---|
| 22:11:15 | | A `terraform plan` over a 70-resource partial state reports 48 genuine attribute diffs. `budget` is the second-largest group at 10 of them, alongside `labels` (11) and `project_funding` (10).
| 22:58:31 | 537 | `terraform apply -auto-approve` #2, the destructive run. 102 budget deletes, 83 label clears, 74 project patches, association replacements. 7 calls fail (5×500, 2×400); terraform renders 14 `Error:` lines. Returns 22:59:52.961 with `843/843` in state | CW + transcript |

### The 39.6-second budget burst

`102 × DELETE /api/v3/budget/{id}`, 22:59:11.815Z through 22:59:51.441Z, 102
unique ids, every one HTTP 200, none recorded in the audit trail.

```
ids: 1-31, 35, 40, 42, 46, 51-53, 56-58, 60-67, 69, 72-77, 79-82, 87, 88,
     99-101, 106, 111, 112, 114-117, 119-124, 126-141, 143, 144, 147,
     149-153
```

Across all 7 retained days this is the only destructive budget operation on the
instance, by any user.

---

## Root cause

The old importer emits project configuration with budgets, funding, and labels
absent by design:

```hcl
resource "kion_project" "Clinical_Research_-_Dev" {
    name                 = "Clinical Research - Dev"
    ou_id                = 18
    permission_scheme_id = 3  # TODO: not returned by Kion API - set … before apply
    # NOTE: budget/funding not imported - add manually if needed
    # TODO: owners could not be inferred from permission mapping …
}
```

`terraform import` on its own is safe. It only reads and writes local state. But
`import` blocks inside a `terraform apply` import and then reconcile: any
collection present on the instance and absent from configuration gets removed.
Applying this configuration was therefore equivalent to asking Kion to delete
every budget, every label assignment, and the association links the importer does
not reproduce.

## Contributing factors

1. **`-auto-approve` against a live instance.** All three applies used it.
   Nothing between the generated config and the instance required a human to look
   at the plan.
2. **An autonomy guard removed the stopping point.** The Stop hook armed at 21:13
   told the session not to pause for the user, and at 22:54 it actively blocked
   the session from stopping, four minutes before the destructive apply. The one
   moment the agent did stop to ask, at 22:50, was on its own initiative, and
   that is the moment the earlier mistake got caught.

---

## Damage inventory

Corrected to the full 22:38:30 through 23:36:24 window and to CloudWatch counts.

### Deleted outright

| endpoint | ops | effect | audited |
|---|---:|---|---|
| `DELETE /api/v3/budget/{id}` | 102 | all budget allocations; funding sources fall to $0.00 / No Projects | no |

### Cleared by empty-array `PUT`, with no DELETE issued

`PUT` replaces a whole collection, so `{"labels": []}` erases it:

```http
PUT /api/v3/project/15/labels   {"labels": []}
→ "set 0 labels on project ID: 15"
```

| endpoint | count | objects |
|---|---:|---|
| `PUT /api/v3/project/{id}/labels` | 33 | 1-16, 18, 19, 20, 24, 31, 37, 38, 39, 56, 59, 60, 61, 63, 64, 65, 69, 70 |
| `PUT /api/v3/cloud-rule/{id}/labels` | 30 | 23, 24, 25, 32, 36, 44, 47, 57, 66, 73, 79-92, 199, 240, 275, 276, 278, 279 |
| `PUT /api/v3/ou/{id}/labels` | 20 | 2-13, 18, 19, 21, 35, 42, 43, 44, 45 |

83 label sets erased. Label *definitions* survive, all 100 records. The
assignments did not.

### Replaced or rewritten

| endpoint | ops | note |
|---|---:|---|
| `POST /api/v3/cft/{id}/owner` | 238 | authorized; genuinely ownerless before |
| `POST /api/v3/iam-policy/{id}/owner` | 238 | the 22:45 loop, unnecessary for an unknown share |
| `POST /api/v3/cloud-rule/{id}/owner` | 176 | the 22:45 loop; roughly 82 already had owners |
| `PATCH /api/v3/iam-policy/{id}` | 99 | partial-field patch |
| `PATCH /api/v3/project/{id}` | 74 | sends no funding or budget field |
| `POST /api/v1/project/{id}/owner` | 74 | |
| `PATCH /api/v3/cft/{id}` | 67 | |
| `PATCH /api/v3/project-cloud-access-role/{id}` | 30 | |
| `POST …/association` (project-CAR 24, ou-CAR 18) | 42 | empty-payload replace |
| `POST /api/v3/ou/{id}/owner` | 12 | |
| `PATCH /api/v3/cloud-rule/{id}` | 6 | |

---

## The audit trail undercounts by 31%

CloudWatch recorded 1,360 write requests from user 128 in the window. The audit
trail holds 940. Five endpoint classes are invisible to it:

| method + endpoint | HTTP | audited | gap |
|---|---:|---:|---:|
| `POST /api/v3/cft/{id}/owner` | 238 | 0 | 238 |
| `DELETE /api/v3/budget/{id}` | 102 | 0 | 102 |
| `POST /api/v1/project/{id}/owner` | 74 | 23 | 51 |
| `PATCH /api/v3/iam-policy/{id}` | 99 | 87 | 12 |
| `POST /api/v3/ou/{id}/owner` | 12 | 0 | 12 |
| `POST /api/v3/cft-parse-params` | 3 | 0 | 3 |
| `PATCH /api/v3/cft/{id}` | 67 | 65 | 2 |
| `POST /api/v1/audit-trail/search` | 2 | 0 | 2 |
| total | 1,360 | 940 | 420 |

`DELETE /api/v3/budget/{id}` writing no audit record is the worst of these.
Budgets are financial objects, and their deletion cannot be reconstructed from
the product's own audit trail.
