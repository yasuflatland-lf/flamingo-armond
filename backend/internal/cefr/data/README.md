# Oxford 3000/5000 CEFR word-list data

`oxford-3000.md` and `oxford-5000.md` are the source of truth for
`Card.cefrLevel` classification. They are generated once from the official
Oxford "by CEFR level" PDFs and committed as markdown; the PDFs are **not**
committed.

- Oxford 3000 (A1–B2): `https://www.oxfordlearnersdictionaries.com/external/pdf/wordlists/oxford-3000-5000/The_Oxford_3000_by_CEFR_level.pdf`
- Oxford 5000 extension (B2–C1): `https://www.oxfordlearnersdictionaries.com/external/pdf/wordlists/oxford-3000-5000/The_Oxford_5000_by_CEFR_level.pdf`

Word lists are © Oxford University Press. The repository owner has confirmed
the embedding of this derived data is acceptable.

## Format

Each file contains one or more `## <LEVEL>` headings followed by `- <key>`
bullets, one normalized key per line. `oxford-3000.md` covers A1–B2;
`oxford-5000.md` covers B2–C1; `cambridge-c2.md` covers C2 (see the Cambridge
EVP C2 section below). Keys are already lowercased, with curly apostrophes
folded to straight, sense disambiguators (`bank (money)`) and homograph
superscripts (`do1`) removed, and comma-separated forms (`a, an`) split into
separate keys. Multi-word entries ("ice cream", "no one") are kept as single
keys for whole-string matching.

## Regeneration

Requires Python 3 with `pypdf` (use a throwaway virtualenv; do not add it to
the project). From a scratch directory:

```bash
python3 -m venv /tmp/pdfvenv && /tmp/pdfvenv/bin/pip install pypdf
cd /tmp/pdfvenv
curl -sSL -A "Mozilla/5.0" -o oxford-3000.pdf "https://www.oxfordlearnersdictionaries.com/external/pdf/wordlists/oxford-3000-5000/The_Oxford_3000_by_CEFR_level.pdf"
curl -sSL -A "Mozilla/5.0" -o oxford-5000.pdf "https://www.oxfordlearnersdictionaries.com/external/pdf/wordlists/oxford-3000-5000/The_Oxford_5000_by_CEFR_level.pdf"
/tmp/pdfvenv/bin/python3 generate_wordlists.py   # script below
```

`generate_wordlists.py`:

```python
from pypdf import PdfReader
import re

POS = {"n.", "v.", "adj.", "adv.", "prep.", "pron.", "det.", "conj.", "exclam.",
       "number", "article", "modal", "auxiliary", "indefinite", "definite",
       "ordinal", "infinitive", "symbol", "marker"}
LEVELS = ["A1", "A2", "B1", "B2", "C1"]
ORDER = {l: i for i, l in enumerate(LEVELS, 1)}


def is_pos(tok):
    tok = tok.strip().rstrip(",")
    return bool(tok) and all(p in POS for p in re.split(r"/", tok) if p)


def norm_key(form):
    form = re.sub(r"\s*\([^)]*\)", "", form)            # drop (sense)
    form = form.replace("’", "'").replace("‘", "'")  # curly -> straight
    form = form.strip().lower()
    toks = form.split()
    while len(toks) > 1 and is_pos(toks[-1]):           # drop trailing POS remnants
        toks.pop()
    return " ".join(toks)


def parse(path):
    reader = PdfReader(path)
    current = None
    out = []
    for page in reader.pages:
        for line in page.extract_text().splitlines():
            line = line.replace("\xa0", " ").strip()
            if not line:
                continue
            if line in LEVELS:
                current = line
                continue
            if line.startswith(("©", "The Oxford")) or "important words" in line \
                    or "useful words" in line or "in English" in line:
                continue
            if current is None:
                continue
            line = re.sub(r"(?<=[A-Za-z])[\d¹²³]+", "", line)  # homograph digits
            line = re.sub(r",(?=[A-Za-z])", ", ", line)                       # missing space
            toks = line.split()
            i = len(toks)
            while i > 1 and is_pos(toks[i - 1]):
                i -= 1
            for form in " ".join(toks[:i]).split(","):
                key = norm_key(form)
                if key and not is_pos(key):
                    out.append((current, key))
    return out


def write(path, rows):
    best = {}
    for level, key in rows:
        if key not in best or ORDER[level] > ORDER[best[key]]:
            best[key] = level
    buckets = {l: sorted(k for k, lv in best.items() if lv == l) for l in LEVELS}
    with open(path, "w") as f:
        for level in LEVELS:
            if not buckets[level]:
                continue
            f.write(f"## {level}\n")
            for key in buckets[level]:
                f.write(f"- {key}\n")
            f.write("\n")
    return len(best)


n3 = write("oxford-3000.md", parse("oxford-3000.pdf"))
n5 = write("oxford-5000.md", parse("oxford-5000.pdf"))
print("oxford-3000.md keys:", n3)
print("oxford-5000.md keys:", n5)
```

Move the two generated `.md` files into this directory.

# Cambridge EVP C2 word-list data

`cambridge-c2.md` is the source of truth for `Card.cefrLevel` C2
classification. Oxford's "by CEFR level" lists top out at C1, so the C2 tier is
derived from the Cambridge English Vocabulary Profile (EVP) C2 word list and
committed as markdown; the PDF is **not** committed.

- English Vocabulary Profile C2 word list: `https://cambridge.buckcenter.edu.ec/wp-content/uploads/2021/02/Level-C2-Word-List.pdf` (EVP / English Profile programme, Cambridge Learner Corpus; electronically compiled by Efthimios Mavrogeorgiadis, toe.gr).

Word data is © English Profile / Cambridge University Press. The repository
owner has confirmed the embedding of this derived data is acceptable.

The C2 list is additionally augmented with a supplementary advanced-vocabulary
list provided by the repository owner. No external attribution is available for
this supplementary list; the owner has confirmed its inclusion is acceptable.
The same append-only rule applies: only words that are absent from the combined
Oxford 3000/5000 sets and not already present in the C2 list are added and
tagged C2, so neither Oxford's A1..C1 judgments nor the existing C2 entries are
altered.

## Methodology (append-only)

`cambridge-c2.md` is built **append-only** on top of the Oxford lists: only EVP
headwords that are **absent** from the combined Oxford 3000/5000 sets are
included and tagged C2. Headwords Oxford already classifies (A1..C1) are skipped
entirely, so Oxford's own level judgments are never overridden. After the diff,
a scrutiny pass removes grammar metalanguage (`noun`, `verb`, `adjective`,
`adverb`, `preposition`, `conjunction` — these appear as uppercase part-of-speech
markers in the PDF that the headword extractor would otherwise mistake for
entries) and a few everyday words that leaked through only because Oxford happens
to omit them (e.g. `number`). The bias is toward keeping: advanced, literary,
technical, and genuine multi-word lexical items (`artificial intelligence`,
`peer pressure`, `side effect`, `immune system`, `disposable income`,
`insofar as`) are retained.

## Regeneration

Requires Python 3 with `pypdf` (use a throwaway virtualenv; do not add it to the
project). Fetch the PDF into a scratch directory, then extract and diff:

```bash
python3 -m venv /tmp/pdfvenv && /tmp/pdfvenv/bin/pip install pypdf
curl -sSL -A "Mozilla/5.0" -o /tmp/cambridge-c2.pdf "https://cambridge.buckcenter.edu.ec/wp-content/uploads/2021/02/Level-C2-Word-List.pdf"
```

The EVP PDF front matter occupies pages 1–6; the alphabetical word body starts
at page 7 (zero-based index 6). For every page from index 6 onward, each text
line is normalized (`\xa0` → space, trimmed) and matched against the
headword-before-IPA pattern

```text
^([A-Za-z][A-Za-z '’\-]*?)\s+/
```

which captures the headword that precedes the IPA transcription (`/.../`). A
match is skipped when the captured headword ends with `:`, exceeds 30
characters, or has more than four words. Each headword is normalized to the
canonical key form (NFC-normalize; lowercase; fold curly `‘ ’` to straight `'`;
strip leading/trailing whitespace, Unicode punctuation, and symbols, preserving
internal whitespace) — identical to `domain.NormalizeWord` so the keys align
with the Oxford keys and the classifier.

The same normalization is applied to the `- key` bullets parsed from
`oxford-3000.md` and `oxford-5000.md`. The diff `EVP_headwords − Oxford_keys`
yields the C2 candidate set; the scrutiny pass above removes the non-C2 leaks;
the surviving keys are written sorted ascending under a single `## C2` heading,
one `- <key>` bullet per line.
