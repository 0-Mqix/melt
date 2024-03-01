package melt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

const OUTPUT_DELAY = time.Millisecond * 100

func (f *Furnace) Output() {
	f.outputTimer.Reset(OUTPUT_DELAY)
}

func (f *Furnace) output() {
	var fileStyles string
	var group sync.WaitGroup

	if f.tailwind {
		group.Add(1)
		go func() {
			f.runTailwind()
			group.Done()
		}()
	}

	if f.style {
		group.Add(1)
		go func() {
			fmt.Println("[MELT] updating: styles")
			var styles string

			for _, c := range f.Components {
				styles += c.Style
			}

			fileStyles = f.transpileStyleFiles()
			styles = fileStyles + f.sortStyles(styles)

			if f.styleOutputFile != "" {
				writeOutputFile(f.styleOutputFile, []byte(styles))
			}

			f.Styles = styles
			group.Done()
		}()
	}

	group.Wait()

	if f.watcherSendReloadEvent {
		f.SendReloadEvent()
	}

	if f.outputFile != "" {

		if f.productionMode {
			fmt.Println("[MELT] output is currently not suported in production mode")
			return
		}

		var output Build

		output.FileStyles = fileStyles
		output.TailwindStyles = f.TailwindStyles

		for _, c := range f.Components {
			output.Components = append(output.Components, c)
		}

		for path := range f.Roots {
			raw, err := os.ReadFile(path)

			if err != nil {
				fmt.Println("[MELT] [BUILD]", err)
				continue
			}

			root, _ := f.createRoot(path, bytes.NewBuffer(raw), false)
			output.Roots = append(output.Roots, root)
		}

		build, err := json.Marshal(output)
		if err != nil {
			fmt.Println("[MELT] [BUILD]", err)
			return
		}

		writeOutputFile(f.outputFile, build)
	}
}
