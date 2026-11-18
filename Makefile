RAW := data/raw
SUBTITLES := $(RAW)/opensubtitles.txt
LEIPZIG_FILES := $(RAW)/ell-gr_web_2015_1M/ell-gr_web_2015_1M-sentences.txt \
	$(RAW)/ell_news_2022_1M/ell_news_2022_1M-sentences.txt \
	$(RAW)/ell_wikipedia_2021_1M/ell_wikipedia_2021_1M-sentences.txt
LEIPZIG := https://downloads.wortschatz-leipzig.de/corpora

.PHONY: build test stress bench model data

build:
	go build -o bin/greeklish ./cmd/greeklish

test:
	go test ./...

stress:
	go test -v -run 'Stress|Speed' .

bench:
	go test -run '^$$' -bench . .

# Retrains model/greeklish.bin and testdata/heldout.txt. Subtitles are
# training only: their license does not allow publishing their sentences.
model: data
	go run ./cmd/train -o model/greeklish.bin -en $(RAW)/en_50k.txt -heldout testdata/heldout.txt \
		-train-only $(SUBTITLES) $(SUBTITLES) $(LEIPZIG_FILES)

data: $(SUBTITLES) $(LEIPZIG_FILES) $(RAW)/en_50k.txt

$(SUBTITLES):
	mkdir -p $(RAW)
	curl -sL https://object.pouta.csc.fi/OPUS-OpenSubtitles/v2018/mono/el.txt.gz | gzip -dc 2>/dev/null | head -n 12000000 > $@

$(RAW)/%-sentences.txt:
	mkdir -p $(RAW)
	curl -sL $(LEIPZIG)/$(notdir $(*D)).tar.gz | tar -xzf - -C $(RAW) --wildcards '*-sentences.txt'

$(RAW)/en_50k.txt:
	mkdir -p $(RAW)
	curl -sL -o $@ https://raw.githubusercontent.com/hermitdave/FrequencyWords/master/content/2018/en/en_50k.txt
