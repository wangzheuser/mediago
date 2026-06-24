package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nichuanfang/medigo/internal/config"
	"github.com/nichuanfang/medigo/internal/cookie"
	"github.com/nichuanfang/medigo/internal/download"
	"github.com/nichuanfang/medigo/internal/extractor"
	"github.com/nichuanfang/medigo/internal/util"
)

type cliOptions struct {
	quality            string
	outputDir          string
	concurrency        int
	cookies            string
	cookiesFromBrowser string
	listOnly           bool
	downloadAll        bool
	jsonOutput         bool
	overwrite          bool
	proxy              string
}

func main() {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cfg := config.Load()
	opts := cliOptions{
		quality:     cfg.Quality,
		outputDir:   cfg.OutputDir,
		concurrency: cfg.Concurrency,
		proxy:       cfg.Proxy,
	}
	if opts.quality == "" {
		opts.quality = "best"
	}
	if opts.outputDir == "" {
		opts.outputDir = "."
	}
	if opts.concurrency <= 0 {
		opts.concurrency = 10
	}

	cmd := &cobra.Command{
		Use:   "medigo [flags] URL",
		Short: "Download videos and course assets from supported sites",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDownload(args[0], opts)
		},
	}
	cmd.PersistentFlags().StringVarP(&opts.quality, "quality", "q", opts.quality, "preferred quality (best/worst/1080p/720p/480p)")
	cmd.PersistentFlags().StringVarP(&opts.outputDir, "output", "o", opts.outputDir, "output directory")
	cmd.PersistentFlags().IntVarP(&opts.concurrency, "concurrency", "c", opts.concurrency, "download concurrency")
	cmd.PersistentFlags().StringVar(&opts.cookies, "cookies", "", "Netscape cookie file path")
	cmd.PersistentFlags().StringVar(&opts.cookiesFromBrowser, "cookies-from-browser", "", "read cookies from browser (chrome/edge/firefox)")
	cmd.PersistentFlags().BoolVar(&opts.listOnly, "list", false, "print extracted media info as JSON without downloading")
	cmd.PersistentFlags().BoolVar(&opts.downloadAll, "all", false, "download all playlist/course entries")
	cmd.PersistentFlags().BoolVar(&opts.jsonOutput, "json", false, "print extracted media info as JSON without downloading")
	cmd.PersistentFlags().BoolVar(&opts.overwrite, "overwrite", false, "overwrite existing output files")
	cmd.PersistentFlags().StringVar(&opts.proxy, "proxy", opts.proxy, "HTTP/SOCKS proxy URL for extractor and downloader requests")

	cmd.AddCommand(newSitesCmd())
	return cmd
}

func newSitesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sites",
		Short: "List supported sites as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(extractor.ListSites())
		},
	}
}

func runDownload(rawURL string, opts cliOptions) error {
	if err := util.SetDefaultProxy(opts.proxy); err != nil {
		return err
	}
	store := cookie.NewStore()
	if opts.cookies != "" {
		if err := store.LoadFromFile(opts.cookies); err != nil {
			return fmt.Errorf("load cookies file: %w", err)
		}
	}
	if opts.cookiesFromBrowser != "" {
		if err := store.LoadFromBrowser(opts.cookiesFromBrowser); err != nil {
			return fmt.Errorf("load browser cookies: %w", err)
		}
	}

	ext, err := extractor.Match(rawURL)
	if err != nil {
		return err
	}
	info, err := ext.Extract(rawURL, &extractor.ExtractOpts{
		Cookies:  store.Jar(),
		Quality:  opts.quality,
		ListOnly: opts.listOnly || opts.jsonOutput,
	})
	if err != nil {
		return err
	}

	if opts.listOnly || opts.jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}

	engine := download.New(download.Opts{
		Concurrency: opts.concurrency,
		OutputDir:   opts.outputDir,
		Overwrite:   opts.overwrite,
	})
	paths, err := downloadSelected(engine, info, opts)
	if err != nil {
		return err
	}
	for _, p := range paths {
		fmt.Fprintln(os.Stdout, p)
	}
	return nil
}

func downloadSelected(engine *download.Engine, info *extractor.MediaInfo, opts cliOptions) ([]string, error) {
	if !info.IsPlaylist() {
		p, err := downloadOne(engine, info, opts.quality)
		if err != nil {
			return nil, err
		}
		return []string{p}, nil
	}

	entries := info.Entries
	if !opts.downloadAll {
		entry := firstDownloadableEntry(entries)
		if entry == nil {
			return nil, fmt.Errorf("playlist %q has no downloadable entries", info.Title)
		}
		p, err := downloadOne(engine, entry, opts.quality)
		if err != nil {
			return nil, err
		}
		return []string{p}, nil
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if len(entry.Streams) == 0 {
			continue
		}
		p, err := downloadOne(engine, entry, opts.quality)
		if err != nil {
			return paths, err
		}
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("playlist %q has no downloadable entries", info.Title)
	}
	return paths, nil
}

func firstDownloadableEntry(entries []*extractor.MediaInfo) *extractor.MediaInfo {
	for _, entry := range entries {
		if entry != nil && len(entry.Streams) > 0 {
			return entry
		}
	}
	return nil
}

func downloadOne(engine *download.Engine, info *extractor.MediaInfo, quality string) (string, error) {
	key, stream := download.SelectBestStream(info.Streams, quality)
	if key == "" {
		return "", fmt.Errorf("%q has no downloadable streams", info.Title)
	}
	return engine.Download(info, stream)
}
