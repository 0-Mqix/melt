package melt

import (
	"sync"
	"sync/atomic"
)

/*
TODO:
  - make readme with documentation
  - fix sass transpiler inside folder
*/

type Furnace struct {
	ComponentComments bool
	AutoReloadEvent   bool
	ReloadEventUrl    string
	PrintRenderOutput bool

	Components map[string]*Component
	Roots      map[string]*Root

	reloadSubscribers map[string]chan bool
	subscribersMutex  sync.Mutex

	lastArgumentId atomic.Int64

	dependencyOf map[string]map[string]bool
}

type MeltOption func(*Furnace)

func New(options ...MeltOption) *Furnace {
	f := &Furnace{
		Components: make(map[string]*Component),
		Roots:      make(map[string]*Root),

		ComponentComments: true,
		PrintRenderOutput: false,

		reloadSubscribers: make(map[string]chan bool),
		dependencyOf:      make(map[string]map[string]bool),
	}

	for _, option := range options {
		option(f)
	}

	return f
}

func WithPrintRenderOutput(value bool) MeltOption {
	return func(f *Furnace) {
		f.PrintRenderOutput = value
	}
}

func WithComponentComments(value bool) MeltOption {
	return func(f *Furnace) {
		f.PrintRenderOutput = value
	}
}

func WithAutoReloadEvent(url string, paths ...string) MeltOption {
	return func(f *Furnace) {
		f.AutoReloadEvent = true
		f.ReloadEventUrl = url

		go f.StartWatcher(paths...)
	}
}
