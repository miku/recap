# RECAP 1                          2026-04-26                           0.1.0

## NAME

recap - summarize text from stdin via an OpenAI-compatible LLM

## SYNOPSIS

`recap` [*OPTIONS*]

## DESCRIPTION

`recap` reads text from standard input and writes an LLM-generated
summary to standard output. It targets any OpenAI-compatible chat
completions endpoint, with optional autodiscovery of a local Ollama
installation.

Several summarization styles are bundled — for articles, lecture or
video transcripts, podcast conversations, and academic papers — and
new ones can be added by dropping markdown templates into the prompt
directory at build time.

Summaries are cached on disk under `$XDG_CACHE_HOME/recap`. Because
LLMs are probabilistic, multiple variants for the same input may
coexist in the cache; on a hit, one is picked at random. The `-A`
flag concatenates every cached variant for an input as a single
markdown document so the outputs of different models and styles can
be compared with a renderer of choice.

## OPTIONS

`-e` *URL*
  OpenAI-compatible base URL. Autodiscovered if empty.

`-k` *KEY*
  API key. Defaults to the value of `OPENAI_API_KEY`.

`-m` *MODEL*
  Model name. Autodiscovered if empty.

`-s` *STYLE*
  Summarization style. Run with `-l` for the list. Default: `basic`.

`-l`
  List available styles and exit.

`-i`
  Show the resolved endpoint, model, styles, and cache directory;
  do not call the LLM.

`-v`
  Verbose. Print resolved configuration, cache state, and timing to
  standard error.

`-f`
  Force a new summary. Skip the cache read but still record the
  result, growing the variant pool for this input.

`-A`
  Render every cached variant for the input on stdin as a single
  concatenated markdown document.

`-t` *DURATION*
  Request timeout (default 5m).

## ENVIRONMENT

`OPENAI_BASE_URL`, `OPENAI_API_KEY`, `OPENAI_MODEL`
  Used when the corresponding flag is unset.

`OLLAMA_HOST`
  Endpoint used for model autodiscovery when no other configuration
  is available. Defaults to `http://localhost:11434`.

## FILES

`$XDG_CACHE_HOME/recap/`
  Cache root. Each input has its own subdirectory whose name is the
  SHA-256 of the input text. Files within are named
  `<modelslug>__<style>__<bodyhash>.md` and contain YAML front
  matter followed by the response body.

## EXAMPLES

Default summary using the autodiscovered model:

    cat article.md | recap

Pick a style tailored to the input type:

    recap -s transcript < lecture.vtt
    recap -s podcast    < interview.txt
    recap -s paper      < paper.txt

Force a fresh variant (the cache grows over repeated runs):

    recap -f < input.txt

Render every cached variant as one markdown document:

    recap -A < input.txt | glow -

Use a specific model on a remote endpoint:

    recap -e https://api.example.com/v1 -k "$TOKEN" -m gpt-4o < article.txt

## AUTHOR

Martin Czygan <martin.czygan@gmail.com>

## SEE ALSO

`glow(1)`, `ollama(1)`
