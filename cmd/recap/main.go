// Command recap reads text from stdin and writes an LLM-generated
// summary to stdout. The endpoint and model are resolved from flags,
// environment, or autodiscovered via Ollama. Results are cached on
// disk; cached variants are picked at random. Use -A to dump every
// cached summary for the current input as one markdown document.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/miku/recap/internal/cache"
	"github.com/miku/recap/internal/discover"
	"github.com/miku/recap/internal/llm"
	"github.com/miku/recap/prompts"
)

func main() {
	var (
		endpoint = flag.String("e", "", "OpenAI-compatible base URL (autodiscovered if empty)")
		apiKey   = flag.String("k", "", "API key (defaults to $OPENAI_API_KEY)")
		model    = flag.String("m", "", "Model name (autodiscovered if empty)")
		style    = flag.String("s", "basic", "Summarization style (see -l)")
		list     = flag.Bool("l", false, "List available styles and exit")
		info     = flag.Bool("i", false, "Show resolved endpoint, model, styles, and cache dir; then exit")
		verbose  = flag.Bool("v", false, "Verbose: print resolved config, cache state, and timing to stderr")
		force    = flag.Bool("f", false, "Force a new summary; bypass cache read but still record the result")
		all      = flag.Bool("A", false, "Render all cached summaries for the input on stdin as one markdown doc")
		timeout  = flag.Duration("t", 5*time.Minute, "Request timeout")
	)
	flag.Usage = usage
	flag.Parse()

	if *list {
		for _, s := range prompts.List() {
			fmt.Println(s)
		}
		return
	}

	cfg, err := discover.Resolve(discover.Options{
		Endpoint: *endpoint,
		APIKey:   *apiKey,
		Model:    *model,
	})
	if err != nil {
		die("%v", err)
	}

	cc, err := cache.New("")
	if err != nil {
		die("cache: %v", err)
	}

	if *info {
		fmt.Printf("endpoint: %s\nmodel:    %s\nstyles:   %s\ncache:    %s\n",
			cfg.Endpoint, cfg.Model, strings.Join(prompts.List(), ", "), cc.Root())
		return
	}

	text, err := io.ReadAll(os.Stdin)
	if err != nil {
		die("read stdin: %v", err)
	}
	if len(text) == 0 {
		die("no input on stdin")
	}

	inputKey := cache.InputKey(string(text))

	if *all {
		entries, err := cc.All(inputKey)
		if err != nil {
			die("cache: %v", err)
		}
		if len(entries) == 0 {
			die("no cached summaries for this input (key %s)", inputKey[:12])
		}
		renderAll(os.Stdout, cc.Dir(inputKey), inputKey, entries)
		return
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "endpoint: %s\nmodel:    %s\nstyle:    %s\ninput:    %s\n",
			cfg.Endpoint, cfg.Model, *style, inputKey[:12])
	}

	rendered, err := prompts.Render(*style, prompts.Data{Text: string(text)})
	if err != nil {
		die("%v", err)
	}

	if !*force {
		variants, err := cc.Variants(inputKey, cfg.Model, *style)
		if err != nil {
			if *verbose {
				fmt.Fprintf(os.Stderr, "cache: read failed: %v\n", err)
			}
		} else if len(variants) > 0 {
			if *verbose {
				fmt.Fprintf(os.Stderr, "cache: hit (1 of %d variants)\n", len(variants))
			}
			fmt.Println(strings.TrimRight(cache.PickRandom(variants).Body, "\n"))
			return
		} else if *verbose {
			fmt.Fprintln(os.Stderr, "cache: miss")
		}
	} else if *verbose {
		existing, _ := cc.Variants(inputKey, cfg.Model, *style)
		fmt.Fprintf(os.Stderr, "cache: forced (%d existing variants for this model+style)\n", len(existing))
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := llm.New(cfg.Endpoint, cfg.APIKey, llm.WithTimeout(*timeout))
	start := time.Now()
	out, err := client.Complete(ctx, cfg.Model, rendered)
	if err != nil {
		die("%v", err)
	}
	elapsed := time.Since(start)
	body := strings.TrimRight(out, "\n")

	entry := cache.Entry{
		Model:       cfg.Model,
		Endpoint:    cfg.Endpoint,
		Style:       *style,
		Created:     time.Now().UTC(),
		Elapsed:     elapsed,
		InputBytes:  len(text),
		OutputBytes: len(body),
		Body:        body,
	}
	path, err := cc.Put(inputKey, entry)
	if err != nil && *verbose {
		fmt.Fprintf(os.Stderr, "cache: write failed: %v\n", err)
	} else if *verbose {
		fmt.Fprintf(os.Stderr, "cache: wrote %s\n", path)
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "took %s (%d bytes in, %d bytes out)\n",
			elapsed.Round(time.Millisecond), len(text), len(body))
	}
	fmt.Println(body)
}

func renderAll(w io.Writer, dir, inputKey string, entries []cache.Entry) {
	fmt.Fprintf(w, "# recap: %d variant(s) for input `%s`\n\n", len(entries), inputKey[:12])
	fmt.Fprintf(w, "_directory: `%s`_\n\n", dir)
	for i, e := range entries {
		title := e.Model
		if e.Style != "" {
			title += " — " + e.Style
		}
		if title == "" {
			title = "(unknown)"
		}
		fmt.Fprintf(w, "## %d. %s\n\n", i+1, title)
		var meta []string
		if !e.Created.IsZero() {
			meta = append(meta, "created: "+e.Created.UTC().Format(time.RFC3339))
		}
		if e.Endpoint != "" {
			meta = append(meta, "endpoint: `"+e.Endpoint+"`")
		}
		if e.Elapsed > 0 {
			meta = append(meta, "elapsed: "+e.Elapsed.Round(time.Millisecond).String())
		}
		if e.OutputBytes > 0 {
			meta = append(meta, fmt.Sprintf("%d bytes", e.OutputBytes))
		}
		if len(meta) > 0 {
			fmt.Fprintf(w, "_%s_\n\n", strings.Join(meta, " · "))
		}
		fmt.Fprintln(w, strings.TrimRight(e.Body, "\n"))
		if i < len(entries)-1 {
			fmt.Fprintln(w, "\n---")
		}
	}
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "recap: "+format+"\n", args...)
	os.Exit(1)
}

func usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintln(out, `recap — summarize text from stdin via an OpenAI-compatible LLM.

Usage:
  recap [flags] < input.txt

The endpoint and model are resolved from flags, then environment
(OPENAI_BASE_URL, OPENAI_API_KEY, OPENAI_MODEL), then Ollama
autodiscovery via $OLLAMA_HOST (default http://localhost:11434),
preferring the most recently modified non-embedding model.

Flags:`)
	flag.PrintDefaults()
	fmt.Fprintln(out, `
Examples:
  # Default summary using the autodiscovered model
  cat article.md | recap

  # Pick a style tailored to the input type
  recap -s transcript < lecture.vtt
  recap -s podcast    < interview.txt
  recap -s paper      < paper.txt

  # Show resolved endpoint, model, styles, cache dir (no LLM call)
  recap -i

  # Force a fresh variant (LLMs are probabilistic; -f grows the cache)
  recap -f < input.txt

  # Render every cached variant for an input as one markdown document
  recap -A < input.txt | glow -

  # Use a specific model on a remote endpoint
  recap -e https://api.example.com/v1 -k "$TOKEN" -m gpt-4o < article.txt`)
}
