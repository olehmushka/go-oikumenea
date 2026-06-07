# Plan: add open question — social-network / messenger references for contact channels

## Context

Person contacts shipped in M12 (**D-PersonContactChannels**) as three multi-valued child
tables under the **person** module: `person_emails`, `person_phones`, `person_call_signs`.
The user wants to record a **future direction** (not a committed design): a person's phone
number or email is often *also* a messenger account (Telegram, WhatsApp, Signal, Viber), and
people additionally have social-network handles (Instagram, LinkedIn, X, Facebook) that stand
on their own. This is "just direction" — so it belongs in the deferred-seam backlog as a
`parked` entry, to be shaped by a future planning session, not built now.

Decided in discussion:
- **Model shape:** *both* — a linkage layer over existing `person_phones` / `person_emails`
  (phone/email-derived messenger reachability) **and** a standalone catalog-typed channel for
  handles independent of any phone/email (social usernames, a Telegram @username with no phone).
- **Scope:** both messengers **and** social networks, distinguished by a catalog type.

## Changes (docs only — `docs/` is the source of truth)

### 1. `docs/open-questions.md` — new entry in the `## person` section

Add **DS-41** (next free number; 40 is last used) after DS-40 (line ~70), matching the
existing *Default / Trigger / `parked`* format:

```
**DS-41 · Social-network / messenger references for contact channels.**
Builds on [D-PersonContactChannels](architecture/decisions.md): today a person's reachability
is email / phone / call-sign only.
- *Default* — no record of which messengers a phone/email is reachable on, and no social-network
  handles.
- *Trigger* — operators want to reach or identify people via messengers / social profiles → two
  additive, non-breaking directions: (a) a **linkage** annotating existing `person_phones` /
  `person_emails` rows with the platforms that number/address is reachable on (phone-derived
  messengers: Telegram / WhatsApp / Signal / Viber); (b) a **standalone** catalog-typed channel
  child table (e.g. `person_social_accounts`, mirroring the email/phone child-table pattern) for
  handles independent of any phone/email (social usernames — Instagram / LinkedIn / X — or a
  Telegram @username with no phone). Platforms are catalog-typed like `person_*_types`. Direction
  only; concrete shape and breadth decided when promoted. `parked`
```

### 2. `docs/modules/person.md` — narrative bullet in `## Open seams / future`

Append a bullet after the DS-40 bullet (line ~349), in the same voice as the others:

```
- **Social-network / messenger references** for contact channels are parked (DS-41) — a future
  additive layer linking existing phones/emails to messengers (Telegram/WhatsApp/Signal) plus a
  standalone catalog-typed channel for independent social handles. Direction only; not modelled yet.
```

## Notes / non-goals

- **Not** a `decisions.md` change — `open-questions.md` is non-binding and nothing is decided.
- **No** new module, table, or migration; no ontology-mapping / glossary / README touch (no new
  entity is introduced — a parked seam isn't a domain entity).
- Keep wording loose per the user's intent ("could be developed and improved… just direction").

## Verification

Run the docs link-coherence check from `CLAUDE.md` (relative links must resolve):

```bash
cd docs && python3 - <<'PY'
import re,os,glob
bad=0
for md in glob.glob('**/*.md',recursive=True):
    base=os.path.dirname(md)
    for m in re.finditer(r'\]\(([^)]+)\)',open(md).read()):
        link=m.group(1).split('#')[0]
        if link and not link.startswith('http') and not os.path.exists(os.path.normpath(os.path.join(base,link))):
            print(f"broken: {md} -> {link}"); bad+=1
print("links OK" if bad==0 else f"{bad} broken")
PY
```

Manual: confirm DS-41 reads cleanly in `open-questions.md` and the person.md bullet cross-references DS-41.
