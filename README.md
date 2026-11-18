# greeklish

Offline CLI that turns Greeklish, unaccented or misspelled Greek, or a mix of both into proper Greek. English words, acronyms, numbers, URLs, emails, paths and code stay as written.

```
$ greeklish "kosta, I ekdosi ine online 1.4.2 na to kanw release sto staging ?"
Κώστα, η έκδοση είναι online 1.4.2 να το κάνω release στο staging;

$ echo "δε ξερω που εισαι;" | greeklish
Δεν ξέρω πού είσαι;
```

One static binary of about 70 MB with the model embedded. It starts in under 0.1 s and converts a word in about 0.2 ms on one core.

## Install

Download an archive for Linux, macOS or Windows from the [releases](https://github.com/alex20465/greeklish/releases), or:

```
go install github.com/alex20465/greeklish/cmd/greeklish@latest
```

## History

Every conversion is appended as one JSON line (time, version, input, output) to a local file, to collect real inputs for later improvement and validation:

| OS | File |
| --- | --- |
| Linux | `$XDG_STATE_HOME/greeklish/history.jsonl`, default `~/.local/state/greeklish/history.jsonl` |
| macOS | `~/Library/Application Support/greeklish/history.jsonl` |
| Windows | `%AppData%\greeklish\history.jsonl` |

The file stays on your machine.

## Build

```
make build   # bin/greeklish with the committed model
make test    # curated cases plus stress tests
make stress  # stress tests with accuracy and error reports
make model   # downloads about 1.5 GB of corpora into data/raw and retrains, about 4 minutes
```

Releases are built by GitHub Actions with GoReleaser when a `v*` tag is pushed.

## How it works

A noisy channel decoder over a word trigram language model:

1. **Candidates.** Each word is matched against the vocabulary through transliteration rules (`rules.go`) in a depth first search pruned by the sorted list of accent-free forms. One typo (deletion, insertion, substitution, transposition) is allowed when no frequent exact reading exists. Greek input goes through the same search with the usual ι/η/υ/ει/οι, ο/ω, ε/αι confusions.
2. **Keep or convert.** Keeping a Latin word costs its English frequency, and a character model tells English from Greeklish. Short all-caps words are acronyms.
3. **Decoding.** Viterbi over all candidates of a sentence picks the most likely Greek sequence under the trigram model (stupid backoff, 50-bit hashed n-grams, 1-byte quantized probabilities).
4. **Grammar.** Final ν (την, στην, δεν), interrogative πού/πώς in questions, casing of sentence starts and proper nouns, the Greek question mark and punctuation spacing.

## Accuracy

Word accuracy on 1,959 held-out news, web and Wikipedia sentences the model never saw (`make stress`):

| Input | Accuracy |
| --- | --- |
| Greeklish, phonetic (i, o, e) | 96.4% |
| Greeklish, orthographic (h, w, ai, ei) | 97.0% |
| Greeklish, lazy (8, 3, w) | 96.7% |
| Greeklish with typos in 15% of words | 95.4% |
| Greeklish mixed with English terms | 96.7% |
| Greek without accents | 97.0% |
| Greek with spelling slips | 97.0% |
| Correct Greek, left unchanged | 99.4% |

Known limits: a trigram cannot resolve every ambiguity (ή/η, οι/η, τις/της), it does not insert missing commas, and loanwords that are also Greek words (τεστ, σπριντ) may be converted.

## Data

The model is trained on:

- The [Leipzig Corpora Collection](https://wortschatz.uni-leipzig.de/en/download/Greek) (news 2022, web 2015, Wikipedia 2021), CC BY. D. Goldhahn, T. Eckart, U. Quasthoff: Building Large Monolingual Dictionaries at the Leipzig Corpora Collection: From 100 to 200 Languages. LREC 2012. The held-out sentences in `testdata/heldout.txt` come from these corpora.
- Greek subtitles from [OpenSubtitles](http://www.opensubtitles.org) via [OPUS](https://opus.nlpl.eu/OpenSubtitles). P. Lison, J. Tiedemann: OpenSubtitles2016: Extracting Large Parallel Corpora from Movie and TV Subtitles. LREC 2016. Used for training only; no subtitle text is published in this repository.
- English word frequencies from [hermitdave/FrequencyWords](https://github.com/hermitdave/FrequencyWords).
