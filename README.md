# recap

**Recap and summarize textual material**. Pipe text in, get a structured summary
out. This tool will only do the summarization. For turning your file into text
use other tools like
[pdftotext](https://www.xpdfreader.com/pdftotext-man.html),
[kreuzberg](https://github.com/kreuzberg-dev/kreuzberg),
[typeout](https://github.com/miku/typeout), ...

[![](static/cotton_compr_24118_sm.gif)](https://etc.usf.edu/clipart/24100/24118/cotton_compr_24118.htm)

## Installation

```
$ go install github.com/miku/recap/cmd/recap@latest
```

Or packaged as [deb or rpm](https://github.com/miku/recap/releases).

## LLM selection

If no explicit LLM endpoint and model are given, **recap** will try to discover
a suitable endpoint and model by looking for typical environment variables,
like `OPENAI_BASE_URL` or `OLLAMA_HOST`, but you can also set endpoint `-e`
and model `-m` explicitly.

To show the discovered endpoint and model:

```
$ recap -i
endpoint: http://chiba:11434/v1
model:    nemotron-3-nano:30b-a3b-fp16
styles:   article, basic, paper, podcast, transcript
cache:    /home/tir/.cache/recap

```

## Your PDF, YouTube videos and any other text as input

```
$ kreuzberg extract testdata/2025.loreslm-1.13.pdf | \
    recap -s article -m qwen3.6:latest | glow

$ typeout https://www.youtube.com/watch?v=S4EsRyZQKEc | \
    recap -s transcript -m qwen3.6:latest | glow
```

See some example rendering/screenshot below.

## Usage examples

```shell
# Default summary using the autodiscovered model
$ cat article.md | recap

# Pick a style tailored to the input type
$ recap -s transcript < lecture.vtt
$ recap -s podcast    < interview.txt
$ recap -s paper      < paper.txt

# Show resolved endpoint, model, styles, cache dir (no LLM call)
$ recap -i

# Force a fresh variant (LLMs are probabilistic; -f grows the cache)
$ recap -f < input.txt

# Render every cached variant for an input as one markdown document
$ recap -A < input.txt | glow -p -

# Use a specific model on a remote endpoint
$ recap -e https://api.example.com/v1 -k "$TOKEN" -m gpt-4o < article.txt
```

![](static/recap-6480361.gif)

## Impressions

![](static/termshot-2025.loreslm-1.13.png)

![](static/termshot-S4EsRyZQKEc.png)
