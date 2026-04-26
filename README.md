# recap

Recap and summarize textual material. Pipe text in, get a structured summary
out. This tool will only do the summarization. For turning your file into text
you other tools like
[pdftotext](https://www.xpdfreader.com/pdftotext-man.html),
[kreuzberg](https://github.com/kreuzberg-dev/kreuzberg),
[typeout](https://github.com/miku/typeout), ...

## Installation

```
$ go install github.com/miku/recap/cmd/recap@latest
```

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

