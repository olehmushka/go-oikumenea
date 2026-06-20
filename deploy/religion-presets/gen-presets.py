#!/usr/bin/env python3
"""Reproducible generator for the M22 religion taxonomy seed (D-Religion, refined).

The curated multi-faith taxonomy that migration `migrations/20260601000023_religion.sql` seeds is the
source of truth for the *shipped* data; this script is the reproducible *recipe* that produced it. It
holds the curated tree (deep Christianity + the major world religions, anchored to Wikidata QIDs) and
can emit it as:

  * `taxa.json`     — the bundled preset (provenance / regeneration / future hermenea import seam)
  * `--sql`         — the per-wave `INSERT … SELECT … FROM (VALUES …)` block, for pasting into the
                      migration when the curated set changes (the migration stays the seed of record).

Design rules honored (D-Religion refined, 2026-06-19):
  * A single recursive tree: every node has a `code`, a `rank` (religion/branch/tradition/
    sub_tradition/denomination), an optional `parent` code (None = a root religion), and an optional
    `wikidata` QID. The closure + denormalized root are derived in SQL, not here.
  * No faith vocabulary is hard-coded in *code* — it is all DATA in `TAXA` / `THEISM` below, editable.
  * Boundary: the tree stops at the major historic churches (denomination rank); a specific governed
    instance (this diocese/parish) is a tenant unit, not a taxon.

Usage:
    python3 deploy/religion-presets/gen-presets.py            # write taxa.json next to this script
    python3 deploy/religion-presets/gen-presets.py --sql      # print the migration INSERT block

This script has NO external dependencies (the curated set is embedded). A future revision may instead
pull the subclass tree from Wikidata (SPARQL on `wdt:P279*` under Q9174 "religion") and reconcile by
QID — left as the documented hermenea-import seam in docs/modules/religion.md.
"""
from __future__ import annotations

import argparse
import json
import os
import sys

# Each entry: (rank, code, name, parent_code | None, wikidata_id | None)
# Ordered parent-first so the SQL waves and the JSON both resolve parents by code.
TAXA: list[tuple[str, str, str, str | None, str | None]] = [
    # ---- religions (roots) ----
    ("religion", "christianity", "Christianity", None, "Q5043"),
    ("religion", "islam", "Islam", None, "Q432"),
    ("religion", "judaism", "Judaism", None, "Q9268"),
    ("religion", "hinduism", "Hinduism", None, "Q9089"),
    ("religion", "buddhism", "Buddhism", None, "Q748"),
    ("religion", "sikhism", "Sikhism", None, "Q9554"),
    ("religion", "jainism", "Jainism", None, "Q9163"),
    ("religion", "bahai", "Bahá'í Faith", None, "Q43066"),
    ("religion", "shinto", "Shinto", None, "Q1409"),
    ("religion", "taoism", "Taoism", None, "Q9598"),
    ("religion", "confucianism", "Confucianism", None, "Q9581"),
    ("religion", "zoroastrianism", "Zoroastrianism", None, "Q9601"),
    ("religion", "atheism", "Atheism", None, "Q7066"),
    ("religion", "agnosticism", "Agnosticism", None, "Q41719"),
    ("religion", "traditional", "Traditional & indigenous religions", None, None),
    ("religion", "other", "Other / unclassified", None, None),

    # ---- branches ----
    ("branch", "catholicism", "Catholicism", "christianity", "Q1841"),
    ("branch", "eastern_orthodoxy", "Eastern Orthodoxy", "christianity", "Q3333484"),
    ("branch", "oriental_orthodoxy", "Oriental Orthodoxy", "christianity", "Q156954"),
    ("branch", "church_of_the_east", "Church of the East", "christianity", "Q470330"),
    ("branch", "protestantism", "Protestantism", "christianity", "Q23540"),
    ("branch", "restorationism", "Restorationism & nontrinitarian", "christianity", "Q1140229"),
    ("branch", "independent_christianity", "Independent / nondenominational", "christianity", None),
    ("branch", "sunni", "Sunni Islam", "islam", "Q7444"),
    ("branch", "shia", "Shia Islam", "islam", "Q9585"),
    ("branch", "ibadi", "Ibadi Islam", "islam", "Q319540"),
    ("branch", "sufism", "Sufism", "islam", "Q9622"),
    ("branch", "ahmadiyya", "Ahmadiyya", "islam", "Q170027"),
    ("branch", "orthodox_judaism", "Orthodox Judaism", "judaism", "Q170238"),
    ("branch", "conservative_judaism", "Conservative Judaism", "judaism", "Q188476"),
    ("branch", "reform_judaism", "Reform Judaism", "judaism", "Q102045"),
    ("branch", "reconstructionist_judaism", "Reconstructionist Judaism", "judaism", "Q1150985"),
    ("branch", "karaite_judaism", "Karaite Judaism", "judaism", "Q484591"),
    ("branch", "vaishnavism", "Vaishnavism", "hinduism", "Q842337"),
    ("branch", "shaivism", "Shaivism", "hinduism", "Q319183"),
    ("branch", "shaktism", "Shaktism", "hinduism", "Q1132099"),
    ("branch", "smartism", "Smartism", "hinduism", "Q707348"),
    ("branch", "theravada", "Theravāda", "buddhism", "Q49003"),
    ("branch", "mahayana", "Mahāyāna", "buddhism", "Q43361"),
    ("branch", "vajrayana", "Vajrayāna", "buddhism", "Q489704"),
    ("branch", "digambara", "Digambara", "jainism", "Q1189537"),
    ("branch", "svetambara", "Śvetāmbara", "jainism", "Q726177"),

    # ---- traditions ----
    ("tradition", "latin_church", "Latin Church", "catholicism", "Q612330"),
    ("tradition", "eastern_catholic", "Eastern Catholic Churches", "catholicism", "Q751392"),
    ("tradition", "lutheranism", "Lutheranism", "protestantism", "Q75809"),
    ("tradition", "reformed", "Reformed (Calvinism)", "protestantism", "Q101849"),
    ("tradition", "anglicanism", "Anglicanism", "protestantism", "Q6423963"),
    ("tradition", "anabaptism", "Anabaptism", "protestantism", "Q104088"),
    ("tradition", "baptist", "Baptist", "protestantism", "Q93191"),
    ("tradition", "methodism", "Methodism", "protestantism", "Q104400"),
    ("tradition", "pentecostalism", "Pentecostalism", "protestantism", "Q170022"),
    ("tradition", "adventism", "Adventism", "protestantism", "Q164359"),
    ("tradition", "holiness", "Holiness movement", "protestantism", "Q1535557"),
    ("tradition", "evangelicalism", "Evangelicalism", "protestantism", "Q170997"),
    ("tradition", "quakerism", "Quakerism (Friends)", "protestantism", "Q170582"),

    # ---- sub-traditions ----
    ("sub_tradition", "hanafi", "Hanafi", "sunni", "Q223097"),
    ("sub_tradition", "maliki", "Maliki", "sunni", "Q207922"),
    ("sub_tradition", "shafii", "Shafiʿi", "sunni", "Q220910"),
    ("sub_tradition", "hanbali", "Hanbali", "sunni", "Q200671"),
    ("sub_tradition", "twelver", "Twelver (Ithnāʿasharī)", "shia", "Q170382"),
    ("sub_tradition", "ismailism", "Ismailism", "shia", "Q179872"),
    ("sub_tradition", "zaidiyyah", "Zaidiyyah", "shia", "Q319618"),
    ("sub_tradition", "hasidic", "Hasidic Judaism", "orthodox_judaism", "Q170581"),
    ("sub_tradition", "modern_orthodox", "Modern Orthodox Judaism", "orthodox_judaism", "Q1426764"),
    ("sub_tradition", "haredi", "Haredi Judaism", "orthodox_judaism", "Q208163"),
    ("sub_tradition", "presbyterianism", "Presbyterianism", "reformed", "Q178169"),
    ("sub_tradition", "congregationalism", "Congregationalism", "reformed", "Q1062789"),
    ("sub_tradition", "continental_reformed", "Continental Reformed", "reformed", "Q1129121"),

    # ---- denominations: the major historic churches/bodies (Christianity-focused per the M22 boundary) ----
    ("denomination", "ecumenical_patriarchate", "Ecumenical Patriarchate of Constantinople", "eastern_orthodoxy", "Q656861"),
    ("denomination", "church_of_greece", "Church of Greece", "eastern_orthodoxy", "Q732221"),
    ("denomination", "russian_orthodox_church", "Russian Orthodox Church", "eastern_orthodoxy", "Q60150"),
    ("denomination", "serbian_orthodox_church", "Serbian Orthodox Church", "eastern_orthodoxy", "Q170377"),
    ("denomination", "romanian_orthodox_church", "Romanian Orthodox Church", "eastern_orthodoxy", "Q463041"),
    ("denomination", "bulgarian_orthodox_church", "Bulgarian Orthodox Church", "eastern_orthodoxy", "Q463848"),
    ("denomination", "georgian_orthodox_church", "Georgian Orthodox Church", "eastern_orthodoxy", "Q1129877"),
    ("denomination", "orthodox_church_of_ukraine", "Orthodox Church of Ukraine", "eastern_orthodoxy", "Q30901814"),
    ("denomination", "orthodox_church_in_america", "Orthodox Church in America", "eastern_orthodoxy", "Q673354"),
    ("denomination", "coptic_orthodox_church", "Coptic Orthodox Church", "oriental_orthodoxy", "Q56183"),
    ("denomination", "armenian_apostolic_church", "Armenian Apostolic Church", "oriental_orthodoxy", "Q102140"),
    ("denomination", "ethiopian_orthodox_tewahedo", "Ethiopian Orthodox Tewahedo Church", "oriental_orthodoxy", "Q260415"),
    ("denomination", "syriac_orthodox_church", "Syriac Orthodox Church", "oriental_orthodoxy", "Q464345"),
    ("denomination", "malankara_orthodox_church", "Malankara Orthodox Syrian Church", "oriental_orthodoxy", "Q1815695"),
    ("denomination", "assyrian_church_of_the_east", "Assyrian Church of the East", "church_of_the_east", "Q178379"),
    ("denomination", "ancient_church_of_the_east", "Ancient Church of the East", "church_of_the_east", "Q1130645"),
    ("denomination", "ukrainian_greek_catholic_church", "Ukrainian Greek Catholic Church", "eastern_catholic", "Q1192126"),
    ("denomination", "maronite_church", "Maronite Church", "eastern_catholic", "Q827512"),
    ("denomination", "melkite_greek_catholic_church", "Melkite Greek Catholic Church", "eastern_catholic", "Q1185801"),
    ("denomination", "chaldean_catholic_church", "Chaldean Catholic Church", "eastern_catholic", "Q656801"),
    ("denomination", "syro_malabar_church", "Syro-Malabar Church", "eastern_catholic", "Q1163901"),
    ("denomination", "armenian_catholic_church", "Armenian Catholic Church", "eastern_catholic", "Q807607"),
    ("denomination", "elca", "Evangelical Lutheran Church in America", "lutheranism", "Q1340004"),
    ("denomination", "lcms", "Lutheran Church – Missouri Synod", "lutheranism", "Q1473773"),
    ("denomination", "southern_baptist_convention", "Southern Baptist Convention", "baptist", "Q815672"),
    ("denomination", "united_methodist_church", "United Methodist Church", "methodism", "Q1446703"),
    ("denomination", "church_of_england", "Church of England", "anglicanism", "Q82708"),
    ("denomination", "episcopal_church_usa", "Episcopal Church (USA)", "anglicanism", "Q1366000"),
    ("denomination", "assemblies_of_god", "Assemblies of God", "pentecostalism", "Q598397"),
    ("denomination", "seventh_day_adventist_church", "Seventh-day Adventist Church", "adventism", "Q104319"),
    ("denomination", "lds_church", "Church of Jesus Christ of Latter-day Saints", "restorationism", "Q19595"),
    ("denomination", "jehovahs_witnesses", "Jehovah's Witnesses", "restorationism", "Q35269"),
]

# Religion-type ("theism") classifications, seeded at the religion level (overridable lower down).
# (taxon_code, classification_code)
THEISM: list[tuple[str, str]] = [
    ("christianity", "monotheistic"),
    ("islam", "monotheistic"),
    ("judaism", "monotheistic"),
    ("sikhism", "monotheistic"),
    ("bahai", "monotheistic"),
    ("zoroastrianism", "monotheistic"),
    ("zoroastrianism", "dualistic"),
    ("hinduism", "monotheistic"),
    ("hinduism", "polytheistic"),
    ("hinduism", "henotheistic"),
    ("hinduism", "monistic"),
    ("buddhism", "nontheistic"),
    ("jainism", "nontheistic"),
    ("jainism", "polytheistic"),
    ("shinto", "polytheistic"),
    ("shinto", "animistic"),
    ("taoism", "pantheistic"),
    ("taoism", "polytheistic"),
    ("confucianism", "nontheistic"),
    ("atheism", "atheistic"),
    ("agnosticism", "agnostic"),
    ("traditional", "animistic"),
    ("traditional", "polytheistic"),
]

RANKS = ["religion", "branch", "tradition", "sub_tradition", "denomination"]
SOURCE = "religion-presets"
SOURCE_VERSION = "2026.06"


def validate() -> None:
    codes = {t[1] for t in TAXA}
    if len(codes) != len(TAXA):
        sys.exit("duplicate taxon code in TAXA")
    for rank, code, _name, parent, _qid in TAXA:
        if rank not in RANKS:
            sys.exit(f"unknown rank {rank!r} for {code}")
        if parent is not None and parent not in codes:
            sys.exit(f"taxon {code} references unknown parent {parent}")
        if rank == "religion" and parent is not None:
            sys.exit(f"root religion {code} must have no parent")
    theism_codes = {t[0] for t in THEISM}
    missing = theism_codes - codes
    if missing:
        sys.exit(f"theism tags reference unknown taxa: {sorted(missing)}")


def as_json() -> dict:
    return {
        "source": SOURCE,
        "source_version": SOURCE_VERSION,
        "ranks": RANKS,
        "taxa": [
            {"rank": r, "code": c, "name": n, "parent": p, "wikidata": q}
            for (r, c, n, p, q) in TAXA
        ],
        "theism": [{"taxon": t, "classification": c} for (t, c) in THEISM],
    }


def sql_literal(s: str | None) -> str:
    if s is None:
        return "NULL"
    return "'" + s.replace("'", "''") + "'"


def as_sql() -> str:
    lines: list[str] = []
    for rank in RANKS:
        wave = [t for t in TAXA if t[0] == rank]
        if not wave:
            continue
        lines.append(f"-- {rank}")
        if rank == "religion":
            lines.append(
                "INSERT INTO oikumenea.religion_taxa (code, name, rank_id, wikidata_id, source, source_version)"
            )
            lines.append(
                f"SELECT v.code, v.name, r.id, v.qid, '{SOURCE}', '{SOURCE_VERSION}'"
            )
            lines.append("FROM (VALUES")
            rows = [f"  ({sql_literal(c)},{sql_literal(n)},{sql_literal(q)})" for (_r, c, n, _p, q) in wave]
            lines.append(",\n".join(rows))
            lines.append(") AS v(code,name,qid)")
            lines.append("JOIN oikumenea.religion_taxon_ranks r ON r.code='religion';")
        else:
            lines.append(
                "INSERT INTO oikumenea.religion_taxa (code, name, rank_id, parent_id, wikidata_id, source, source_version)"
            )
            lines.append(
                f"SELECT v.code, v.name, rk.id, p.id, v.qid, '{SOURCE}', '{SOURCE_VERSION}'"
            )
            lines.append("FROM (VALUES")
            rows = [
                f"  ({sql_literal(p)},{sql_literal(c)},{sql_literal(n)},{sql_literal(q)})"
                for (_r, c, n, p, q) in wave
            ]
            lines.append(",\n".join(rows))
            lines.append(") AS v(parent_code,code,name,qid)")
            lines.append(f"JOIN oikumenea.religion_taxon_ranks rk ON rk.code='{rank}'")
            lines.append("JOIN oikumenea.religion_taxa p ON p.code=v.parent_code AND p.deleted_at IS NULL;")
        lines.append("")
    return "\n".join(lines)


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--sql", action="store_true", help="print the migration INSERT block instead of writing JSON")
    args = ap.parse_args()
    validate()
    if args.sql:
        print(as_sql())
        return
    out = os.path.join(os.path.dirname(os.path.abspath(__file__)), "taxa.json")
    with open(out, "w", encoding="utf-8") as f:
        json.dump(as_json(), f, ensure_ascii=False, indent=2)
        f.write("\n")
    print(f"wrote {out} ({len(TAXA)} taxa, {len(THEISM)} theism tags)")


if __name__ == "__main__":
    main()
