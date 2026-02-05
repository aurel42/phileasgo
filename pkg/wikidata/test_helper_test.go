package wikidata

import (
	"log/slog"
	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/store"
)

func newTestPipeline(st store.Store) *Pipeline {
	cfg := config.NewProvider(&config.Config{}, nil)
	dm, _ := NewDensityManager("../../configs/languages.yaml")
	if dm == nil {
		// Fallback for tests if file not found
		dm = &DensityManager{languages: make(map[string]LanguageConfig)}
	}

	return NewPipeline(
		st,
		&MockWikidataClient{},
		&MockWikipediaProvider{},
		&geo.Service{},
		poi.NewManager(cfg, st, nil),
		NewGrid(),
		NewLanguageMapper(st, nil, slog.Default()),
		&MockClassifier{},
		dm,
		cfg,
		slog.Default(),
	)
}
