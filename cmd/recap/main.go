// Command recap reads text from stdin and writes an LLM-generated
// summary to stdout. The endpoint and model are resolved from flags,
// environment, or autodiscovered via Ollama.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

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
		info     = flag.Bool("i", false, "Show resolved endpoint, model, and styles and exit")
		verbose  = flag.Bool("v", false, "Verbose: print resolved config and timing to stderr")
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

	if *info {
		fmt.Printf("endpoint: %s\nmodel:    %s\nstyles:   %s\n",
			cfg.Endpoint, cfg.Model, strings.Join(prompts.List(), ", "))
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

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := llm.New(cfg.Endpoint, cfg.APIKey, llm.WithTimeout(*timeout))
	start := time.Now()
	out, err := client.Complete(ctx, cfg.Model, rendered)
	if err != nil {
		die("%v", err)
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
