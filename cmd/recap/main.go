// Command recap reads text from stdin and writes an LLM-generated
// summary to stdout. The endpoint and model are resolved from flags,
// environment, or autodiscovered via Ollama. Results are cached on
// disk; cached variants are picked at random.
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
		timeout  = flag.Duration("t", 5*time.Minute, "Request timeout")
	)
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

	if *verbose {
		fmt.Fprintf(os.Stderr, "endpoint: %s\nmodel:    %s\nstyle:    %s\n",
			cfg.Endpoint, cfg.Model, *style)
	}

	text, err := io.ReadAll(os.Stdin)
	if err != nil {
		die("read stdin: %v", err)
	}
	if len(text) == 0 {
		die("no input on stdin")
	}

	rendered, err := prompts.Render(*style, prompts.Data{Text: string(text)})
	if err != nil {
		die("%v", err)
	}

	key := cache.Key(cfg.Endpoint, cfg.Model, rendered)

	if !*force {
		if hit, ok, err := cc.Get(key); err != nil {
			if *verbose {
				fmt.Fprintf(os.Stderr, "cache: read failed: %v\n", err)
			}
		} else if ok {
			if *verbose {
				names, _ := cc.List(key)
				fmt.Fprintf(os.Stderr, "cache: hit (1 of %d variants)\n", len(names))
			}
			fmt.Println(hit)
			return
		} else if *verbose {
			fmt.Fprintln(os.Stderr, "cache: miss")
		}
	} else if *verbose {
		names, _ := cc.List(key)
		fmt.Fprintf(os.Stderr, "cache: forced (%d existing variants)\n", len(names))
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := llm.New(cfg.Endpoint, cfg.APIKey, llm.WithTimeout(*timeout))
	start := time.Now()
	out, err := client.Complete(ctx, cfg.Model, rendered)
	if err != nil {
		die("%v", err)
	}

	if err := cc.Put(key, out); err != nil && *verbose {
		fmt.Fprintf(os.Stderr, "cache: write failed: %v\n", err)
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "took %s (%d bytes in, %d bytes out)\n",
			time.Since(start).Round(time.Millisecond), len(text), len(out))
	}
	fmt.Println(out)
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "recap: "+format+"\n", args...)
	os.Exit(1)
}
