package melt

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	fs "github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
)

var RELOAD_EVENT = []byte("event:message\ndata:reload\n\n")

const DELAY = time.Duration(100 * time.Microsecond)

//TODO:
// - handle delete

func (f *Furnace) Update(path string) {

	path = strings.ToLower(path)
	fmt.Println("[MELT] updating:", path)

	_, ok := f.GetComponent(path, true)

	if !ok {
		return
	}

	list, ok := f.dependencyOf[path]

	if ok {
		for path := range list {
			f.Update(path)
		}
	}
}

func handleEvent(e fs.Event, f *Furnace) func() {
	return func() {
		if strings.HasSuffix(e.Name, ".html") {
			if _, ok := f.Roots[e.Name]; ok {
				f.GetRoot(e.Name, true)
			} else {
				f.Update(e.Name)
			}

			f.SendReloadEvent()
		}
	}
}

func (f *Furnace) StartWatcher(paths ...string) {

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
					return
				}

				timer, ok := updates[e.Name]

				if !ok {
					updates[e.Name] = time.AfterFunc(DELAY, handleEvent(e, f))
				} else {
					timer.Reset(DELAY)
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}

				panic(fmt.Sprintf("[MELT] watcher: %v", err))
			}
		}
	}()

	for _, path := range paths {
		err = watcher.Add(path)

		if err != nil {
			fmt.Println("[MELT] watcher:", err)
		}
	}

	select {}
}

func (f *Furnace) SendReloadEvent() {
	for _, c := range f.reloadSubscribers {
		c <- true
	}
}

func (f *Furnace) ReloadPageServerEvent(w http.ResponseWriter, r *http.Request) {
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
