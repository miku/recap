# recap

Recap and summarize textual material. Pipe text in, get a structured summary
out.

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


