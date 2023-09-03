package melt

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	fs "github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
)

var (
	RELOAD_EVENT = []byte("event:message\ndata:reload\n\n")

	mutex   sync.Mutex
	created struct {
		component *Component
		path      string
		raw       []byte
	}
)

const DELAY = time.Duration(50 * time.Millisecond)

func hasExtention(path string, extentions []string) bool {
	extention := filepath.Ext(path)

	for _, x := range extentions {
		if extention == x {
			return true
		}
	}

	return false
}

func (f *Furnace) update(path string) {
	fmt.Println("[MELT] updating:", path)

	_, ok := f.GetComponent(path, true)

	if !ok {
		return
	}

	f.updateDependencies(path)
}

func (f *Furnace) updateDependencies(path string) {
	list, ok := f.dependencyOf[path]

	if ok {
		for path := range list {
			f.update(path)
		}
	}
}

func handleEvent(e fs.Event, f *Furnace) func() {
	return func() {

		mutex.Lock()
		defer mutex.Unlock()

		path := formatPath(e.Name)
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		root := name == "root" || strings.HasPrefix(name, "root_") || strings.HasPrefix(name, "root-")

		if e.Has(fs.Write) {
			if root {
				f.GetRoot(path, true)
				fmt.Println("[MELT] updating:", path)
			} else {
				f.update(path)
			}

		} else if e.Has(fs.Create) {
			if root {
				f.GetRoot(path, true)

			} else if c, ok := f.GetComponent(path, true); ok {
				created.component = c
				created.path = path
				created.raw, _ = os.ReadFile(path)
				f.updateDependencies(path)
			}

		} else if e.Has(fs.Rename) || e.Has(fs.Remove) {

			if root {
				delete(f.Roots, path)
				return
			}

			c, ok := f.Components[path]

			if !ok {
				return
			}

			if created.component == nil {
				if e.Has(fs.Remove) {
					delete(f.dependencyOf, path)
					delete(f.Components, path)
				}

				return
			}

			new, err := f.Render(ComponentName(path), bytes.NewBuffer(created.raw), path)

			if err != nil || c.String != new.String {
				return
			}

			fmt.Printf("[MELT] fixing paths %s -> %s\n", path, created.path)

			list, ok := f.dependencyOf[path]

			if ok && f.AutoUpdateImports {
				for dependency := range list {
					updateImportPath(dependency, path, created.path)
				}

				delete(f.dependencyOf, path)
				delete(f.Components, path)
			}
		}

		f.SendReloadEvent()
	}
}

func (f *Furnace) StartWatcher(extentions []string, paths ...string) {
	if f.productionMode {
		fmt.Println("[MELT] watcher is disabled in production")
		return
	}

	watcher, err := fs.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	defer watcher.Close()

	updates := make(map[string]*time.Timer)

	go func() {
		for {
			select {
			case e, ok := <-watcher.Events:

				if !ok {
					continue
				}

				path := formatPath(e.Name)

				if !hasExtention(path, extentions) || path == f.StyleOutputFile || path == f.OutputFile {
					continue
				}

				event := fmt.Sprintf("%s-%s", e.Name, e.Op.String())
				timer, ok := updates[event]

				if !ok {
					updates[event] = time.AfterFunc(DELAY, handleEvent(e, f))
				} else {
					timer.Reset(DELAY)
				}

			case err, ok := <-watcher.Errors:

				if !ok {
					continue
				}

				panic(fmt.Sprintf("[MELT] watcher: %v", err))
			}
		}
	}()

	for _, path := range paths {
		filepath.Walk(path, func(path string, info os.FileInfo, _ error) error {
			if info.IsDir() {
				err = watcher.Add(path)

				if err != nil {
					fmt.Println("[MELT] watcher:", err)
				}
			}
			return nil
		})
	}

	fmt.Println("[MELT] watching paths:")
	for _, path := range watcher.WatchList() {
		fmt.Printf("-> %s\n", formatPath(path))
	}

	select {}
}

func (f *Furnace) SendReloadEvent() {
	if f.productionMode {
		fmt.Println("[MELT] reload events are disabled in production")
		return
	}

	for _, c := range f.reloadSubscribers {
		c <- true
	}
}

func (f *Furnace) ReloadEventHandler(w http.ResponseWriter, r *http.Request) {
	if f.productionMode {
		fmt.Println("[MELT] reload events are disabled in production")
		w.WriteHeader(500)
		return
	}

	events := make(chan bool)
	id := uuid.NewString()

	f.subscribersMutex.Lock()
	f.reloadSubscribers[id] = events
	f.subscribersMutex.Unlock()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	defer func() {
		f.subscribersMutex.Lock()
		delete(f.reloadSubscribers, id)
		f.subscribersMutex.Unlock()
	}()

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher.Flush()

	for {
		select {
		case <-events:
			w.Write(RELOAD_EVENT)
			flusher.Flush()

		case <-r.Context().Done():
			return
		}
	}
}

func updateImportPath(file, old, new string) error {
	fmt.Println(file, old, new)

	f, err := os.OpenFile(file, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	content, err := io.ReadAll(f)

	if err != nil {
		return err
	}

	directory := filepath.Dir(file)

	modified := ImportRegex.ReplaceAllFunc(content, func(b []byte) []byte {

		data := ImportRegex.FindStringSubmatch(string(b))
		path := formatPath(filepath.Join(directory, data[3]))

		if path != old {
			return b
		}

		relative, err := filepath.Rel(directory, new)

		if err != nil {
			fmt.Println("[MELT] failed updating path")
			return b
		}

		return []byte(data[1] + data[2] + " " + formatPath(relative) + data[4])
	})

	_, err = f.Seek(0, 0)
	if err != nil {
		return err
	}

	_, err = f.Write(modified)
	if err != nil {
		return err
	}

	err = f.Truncate(int64(len(modified)))
	if err != nil {
		return err
	}

	return nil
}
