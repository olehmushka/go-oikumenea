#!/usr/bin/env python3
"""Generate the bundled language presets for M18 (D-Languages).

Produces, reproducibly, from pinned upstream releases:

  * glottolog-5.3.json  — the full Glottolog 5.3 languoid forest (~26k family/language/dialect nodes
    with immediate parent, ISO 639-3, macroarea, representative point, AES endangerment, countries),
    the canonical `language-scheme` import payload (a JSON array the hermenea glottolog mapper reads).
  * cldr-scripts.json   — CLDR language→writing-system links (which ISO-15924 script a language uses,
    and whether it is primary), the canonical `language-scripts` import payload.

Sources (all CC-BY-4.0 / Unicode license; attribution carried in the source files):
  * Glottolog 5.3 as CLDF — github.com/glottolog/glottolog-cldf  (languages.csv + values.csv)
  * Unicode CLDR languageData — github.com/unicode-org/cldr      (supplementalData.xml)
  * ISO 639-3 / 639-1 code table — iso639-3.sil.org              (iso-639-3.tab, for 2→3 mapping)

Usage:
    python3 deploy/language-presets/gen-presets.py            # download + write both JSON files
    GLOTTOLOG_CACHE=/tmp python3 .../gen-presets.py           # reuse cached downloads in $GLOTTOLOG_CACHE

Network is only needed to (re)download; cached files in $GLOTTOLOG_CACHE are reused when present.
"""
import csv
import io
import json
import os
import sys
import urllib.request
import xml.etree.ElementTree as ET

HERE = os.path.dirname(os.path.abspath(__file__))
CACHE = os.environ.get("GLOTTOLOG_CACHE", "")

GLOTTOLOG_VERSION = "5.3"
LANGUAGES_URL = "https://raw.githubusercontent.com/glottolog/glottolog-cldf/master/cldf/languages.csv"
VALUES_URL = "https://raw.githubusercontent.com/glottolog/glottolog-cldf/master/cldf/values.csv"
CLDR_SUPPL_URL = "https://raw.githubusercontent.com/unicode-org/cldr/main/common/supplemental/supplementalData.xml"
ISO639_URL = "https://iso639-3.sil.org/sites/iso639-3/files/downloads/iso-639-3.tab"


def fetch(url, cache_name):
    """Return the bytes of url, preferring a cached copy in $GLOTTOLOG_CACHE."""
    if CACHE:
        p = os.path.join(CACHE, cache_name)
        if os.path.exists(p):
            with open(p, "rb") as f:
                return f.read()
    sys.stderr.write(f"downloading {url}\n")
    with urllib.request.urlopen(url, timeout=120) as r:
        data = r.read()
    if CACHE:
        with open(os.path.join(CACHE, cache_name), "wb") as f:
            f.write(data)
    return data


def gen_glottolog():
    langs = {}
    rdr = csv.DictReader(io.StringIO(fetch(LANGUAGES_URL, "glang.csv").decode("utf-8")))
    for r in rdr:
        langs[r["ID"]] = r

    parent = {}
    status = {}
    rdr = csv.DictReader(io.StringIO(fetch(VALUES_URL, "values.csv").decode("utf-8")))
    for r in rdr:
        pid = r["Parameter_ID"]
        if pid == "classification":
            path = (r["Value"] or "").strip()
            if path:
                parent[r["Language_ID"]] = path.split("/")[-1]
        elif pid == "aes":
            code = r["Code_ID"] or ""
            if code.startswith("aes-"):
                status[r["Language_ID"]] = code[len("aes-"):]

    records = []
    for gc, r in langs.items():
        rec = {"code": gc, "level": r["Level"], "name": r["Name"]}
        if gc in parent:
            rec["parent"] = parent[gc]
        iso = (r.get("ISO639P3code") or "").strip()
        if iso:
            rec["iso639_3"] = iso.lower()
        ma = (r.get("Macroarea") or "").strip()
        if ma:
            rec["macroarea"] = ma
        for key, col in (("latitude", "Latitude"), ("longitude", "Longitude")):
            v = (r.get(col) or "").strip()
            if v:
                try:
                    rec[key] = float(v)
                except ValueError:
                    pass
        rec["status"] = status.get(gc, "not_endangered")
        countries = [c.strip().upper() for c in (r.get("Countries") or "").split(";") if c.strip()]
        if countries:
            rec["countries"] = countries
        records.append(rec)

    records.sort(key=lambda x: x["code"])
    # The Glottolog version travels via the canonical-envelope sourceVersion, not per-record.
    out = os.path.join(HERE, "glottolog-5.3.json")
    with open(out, "w") as f:
        json.dump(records, f, ensure_ascii=False, separators=(",", ":"))
        f.write("\n")
    sys.stderr.write(f"wrote {out}: {len(records)} languoids\n")


def gen_cldr_scripts():
    # 2-letter (ISO 639-1) -> 3-letter (ISO 639-3) map.
    part1_to_id = {}
    ids = set()
    rdr = csv.DictReader(io.StringIO(fetch(ISO639_URL, "iso639.tab").decode("utf-8")), delimiter="\t")
    for r in rdr:
        ids.add(r["Id"].lower())
        p1 = (r.get("Part1") or "").strip().lower()
        if p1:
            part1_to_id[p1] = r["Id"].lower()

    def to_iso3(subtag):
        s = subtag.strip().lower()
        if len(s) == 2:
            return part1_to_id.get(s)
        if len(s) == 3 and s in ids:
            return s
        return None

    root = ET.fromstring(fetch(CLDR_SUPPL_URL, "suppl.xml").decode("utf-8"))
    primary = {}    # iso3 -> ordered list of scripts (primary entry)
    secondary = {}  # iso3 -> set of scripts (secondary entry)
    for el in root.iter("language"):
        scripts = (el.get("scripts") or "").split()
        if not scripts:
            continue
        iso3 = to_iso3(el.get("type") or "")
        if not iso3:
            continue
        if el.get("alt") == "secondary":
            secondary.setdefault(iso3, set()).update(scripts)
        else:
            primary.setdefault(iso3, [])
            for sc in scripts:
                if sc not in primary[iso3]:
                    primary[iso3].append(sc)

    records = []
    seen = set()
    for iso3, scripts in primary.items():
        for i, sc in enumerate(scripts):
            key = (iso3, sc)
            if key in seen:
                continue
            seen.add(key)
            records.append({"iso639_3": iso3, "writingSystem": sc, "isPrimary": i == 0})
    for iso3, scripts in secondary.items():
        for sc in scripts:
            key = (iso3, sc)
            if key in seen:
                continue
            seen.add(key)
            records.append({"iso639_3": iso3, "writingSystem": sc, "isPrimary": False})

    records.sort(key=lambda x: (x["iso639_3"], x["writingSystem"]))
    out = os.path.join(HERE, "cldr-scripts.json")
    with open(out, "w") as f:
        json.dump(records, f, ensure_ascii=False, separators=(",", ":"))
        f.write("\n")
    sys.stderr.write(f"wrote {out}: {len(records)} language→script links\n")


if __name__ == "__main__":
    gen_glottolog()
    gen_cldr_scripts()
