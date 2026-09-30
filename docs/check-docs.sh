#!/usr/bin/env bash
# check-docs.sh — enforce the invariants the old 54,465-line corpus declared and violated.
#
# That set listed seven "re-checkable" invariants; three were false as written. A waitlist survived
# the decision that deleted it, the subscription enum was defined in six places, and sixteen files had
# no Status: line (five of them top-authority). Root cause: only FILE-level link checking existed, so
# the invariants were prose, not law.
#
# Two detectors, because neither alone is sufficient:
#   outside_owner  a banned mechanism may be named only in the document recording its deletion.
#                  Catches revival in the wrong document; cannot catch careless prose INSIDE the owner.
#   unnegated      a paragraph naming a phrase must also forbid it. Catches assertion anywhere.
#   STRICT negation is used for data-loss and market rules: "deleted"/"removed" are themselves the
#   hostile verbs, so admitting them would let "Accounts are deleted after dormancy" pass.
#
# Requires GNU awk (paragraph records, IGNORECASE) and GNU grep.  cd combi/docs && ./check-docs.sh
set -uo pipefail
cd "$(dirname "$0")"

fail=0; pass=0
ok()  { printf 'PASS  %s\n' "$1"; pass=$((pass+1)); }
bad() { printf 'FAIL  %s\n' "$1"; fail=$((fail+1)); }
show(){ printf '%s\n' "$1" | sed 's/^/          /' | head -n 8; }

# STRICT: genuine prohibitions only. Deliberately excludes deleted|removed|withdrawn|false|claimed.
STRICT='never|(^|[^[:alpha:]])not([^[:alpha:]]|$)|(^|[^[:alpha:]])no([[:space:]]|$)|(^|[^[:alpha:]])none([^[:alpha:]]|$)|only|prohibit|forbid|(^|[^[:alpha:]])ban([^[:alpha:]]|$)|must not|may not|without|absent|refus|avoid|prevent|(^|[^[:alpha:]])kill([^[:alpha:]]|$)|(^|[^[:alpha:]])halt([^[:alpha:]]|$)|reject|cannot|rather than|instead of|except|stronger than|nowhere|free of|exempt'
# BROAD: STRICT plus tombstone words, for mechanisms whose owner may legitimately say "deleted".
BROAD="$STRICT|deleted|removed|withdrawn|tombstone|historical|legacy"

outside_owner() {           # outside_owner PATTERN OWNER.md...
  local pat="$1"; shift
  local owners; owners="^($(IFS='|'; echo "$*"))$"
  grep -inE "$pat" *.md 2>/dev/null | awk -F: -v re="$owners" '$1 !~ re'
}

# unnegated PATTERN NEGSET -> paragraphs that name a phrase while forbidding nothing
unnegated() {
  awk -v RS='' -v IGNORECASE=1 -v pat="$1" -v neg="$2" '
    { gsub(/[*`_]/, "") }
    $0 ~ pat && $0 !~ neg {
      nn = split($0, ln, "\n"); w = 1
      for (i = 1; i <= nn; i++) if (ln[i] ~ pat) { w = i; break }
      s = ln[w]; gsub(/^[ \t>]+|[ \t]+$/, "", s)
      if (length(s) == 0) { s = ln[1]; gsub(/^[ \t>]+|[ \t]+$/, "", s) }
      printf "%s: %s\n", FILENAME, substr(s, 1, 110)
    }' *.md
}

# both PATTERN OWNERS... NEGSET -> union of the two detectors
both() {
  local pat="$1"; shift
  local owners_csv="$1"; shift
  local out o
  out=$(outside_owner "$pat" $(echo "$owners_csv" | tr ',' ' ') 2>/dev/null)
  o=$(unnegated "$pat" "$1")
  printf '%s\n%s\n' "$out" "$o" | grep . || true
}


# 1. The document set is exactly the approved files. Adding one requires deleting one.
stray=$(ls *.md | grep -Ev '^(README|[0-9]{2}-[A-Z0-9-]+)\.md$' || true)
[ -z "$stray" ] && ok "1. only approved document files present" \
                || { bad "1. unapproved file(s): $stray"; show "Add a section, or delete a paragraph elsewhere."; }

# 2. Every document declares Status: within its first 12 lines.
missing=""; for f in *.md; do head -n 12 "$f" | grep -q '\*\*Status:\*\*' || missing="$missing $f"; done
[ -z "$missing" ] && ok "2. every document declares a Status:" || bad "2. no Status: in:$missing"

# 3. The retired readiness vocabulary may not be used as a live status (bold or plain form).
live=$(grep -lE '^(\*\*)?Status:(\*\*)?.*(AGREED|safe to build from)' *.md 2>/dev/null | tr '\n' ' ')
[ -z "$live" ] && ok "3. no document self-declares AGREED / build-readiness" \
               || bad "3. retired status vocabulary in: $live"

# 4. One definition site per state enum: 02-DOMAIN.md section 8.
for m in SUBSCRIPTION MEMBERSHIP TRANSACTION CAPABILITY IDENTIFIER; do
  hits=$(grep -lE "^[[:space:]]*${m}[[:space:]]+[A-Z]+( ·[[:space:]]*[A-Z]+)+" *.md 2>/dev/null | sort | tr '\n' ' ')
  [ "$hits" = "02-DOMAIN.md " ] && ok "4. ${m} enum defined only in 02-DOMAIN.md" \
                               || bad "4. ${m} defined in: ${hits:-<none - check the enum block>}"
done

# 5. Archetype set: no fifth archetype revived as a model term (D-01).
arch=$(outside_owner 'archetype[s]?[^.]{0,40}(individual|personal)' 01-DECISIONS.md 08-STATUS.md README.md)
[ -z "$arch" ] && ok "5. no fifth archetype revived as a model term (D-01)" || { bad "5. archetype drift"; show "$arch"; }

# 6. The waitlist is dead (D-10): named only in its tombstone, and never asserted as behaviour.
wl=$(both 'waitlist|wait-list|long.list|reactivation.queue' '01-DECISIONS.md,08-STATUS.md,README.md,09-PLAN-90D.md' "$BROAD")
[ -z "$wl" ] && ok "6. no waitlist / long-list / reactivation queue (D-10)" || { bad "6. waitlist revived"; show "$wl"; }

# 7. Nothing deleted or reclaimed for dormancy / non-payment — STRICT, anywhere (D-10).
dl=$(unnegated '(delet[[:alpha:]]*|reclaim[[:alpha:]]*|releas[[:alpha:]]*)[^.]{0,45}(dormanc[[:alpha:]]*|non-payment|payment failure|lapse|inactiv[[:alpha:]]*)' "$STRICT")
[ -z "$dl" ] && ok "7. no data-loss-for-lapse asserted anywhere (D-10)" || { bad "7. data-loss-for-lapse language"; show "$dl"; }

# 8. No price escalating with platform size (D-12), except inside its own rejection.
esc=$(both '(doubl[[:alpha:]]*|escalat[[:alpha:]]*)[^.]{0,35}(million|platform size|user count|per user)' '01-DECISIONS.md,08-STATUS.md,README.md' "$BROAD")
[ -z "$esc" ] && ok "8. pricing curve stays flat + metered + capped (D-12)" || { bad "8. size-escalating price revived"; show "$esc"; }

# 9. No unsourced market sizing anywhere — STRICT, so prose cannot launder a TAM.
mk=$(unnegated '(^|[^[:alpha:]])TAM([^[:alpha:]]|$)|market size|billion (people|users|africans)|market share of' "$STRICT")
[ -z "$mk" ] && ok "9. no unsourced market sizing" || { bad "9. market claim without a source"; show "$mk"; }

# 10. Every relative .md link resolves to a real file.
broken=""
for f in *.md; do
  while read -r lk; do
    tgt=${lk%%#*}; tgt=${tgt//%20/ }
    [ -z "$tgt" ] && continue
    [ -e "$tgt" ] || broken="$broken
          $f -> $lk"
  done < <(grep -oE '\]\([^)]+\.md[^)]*\)' "$f" 2>/dev/null | sed 's/^\](//; s/)$//')
done
[ -z "$broken" ] && ok "10. every relative .md link resolves" || { bad "10. broken links:"; printf '%s\n' "$broken"; }

# 11. Banned product vocabulary may appear only in a paragraph that forbids it (D-05).
vok=$(unnegated '(feed|feeds|follower|followers|likes|streak|streaks)' "$BROAD")
[ -z "$vok" ] && ok "11. banned vocabulary used only in forbidding context (D-05)" || { bad "11. banned vocabulary in use"; show "$vok"; }

echo
if [ "$fail" -eq 0 ]; then
  printf 'Docs clean - %d checks passed. These are now enforced, not intended.\n' "$pass"
  exit 0
fi
printf '%d check(s) FAILED (%d passed). Fix the documents; do not relax the check.\n' "$fail" "$pass"
exit 1

echo "== Combi documentation invariants =="
