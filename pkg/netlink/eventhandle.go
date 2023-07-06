/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package netlink

import (
	"errors"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/vishvananda/netlink"
)

type LinkEventHandler struct {
	Logger        log.Logger
	mutex         *sync.Mutex
	stopWg        *sync.WaitGroup
	ifaceChannels map[string]chan<- netlink.LinkUpdate
	stop          chan interface{}
}

func (l *LinkEventHandler) Start() {
	l.mutex = &sync.Mutex{}
	l.ifaceChannels = make(map[string]chan<- netlink.LinkUpdate)
	l.stop = make(chan interface{})
	l.stopWg = &sync.WaitGroup{}
	l.stopWg.Add(1)
	go l.handleEvents()
	level.Info(l.Logger).Log("monitor", "link monitor started")
}

func (l *LinkEventHandler) Subscribe(eventChannel chan<- netlink.LinkUpdate, ifNames ...string) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if eventChannel == nil {
		return errors.New("event channel can not be empty")
	}
	for _, ifName := range ifNames {
		l.ifaceChannels[ifName] = eventChannel
	}
	return nil
}

func (l *LinkEventHandler) Unsubscribe(ifNames ...string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	for _, ifName := range ifNames {
		delete(l.ifaceChannels, ifName)
	}
}

func (l *LinkEventHandler) StopHandler() {
	close(l.stop)
	l.stopWg.Wait()
}

func (l *LinkEventHandler) handleEvents() {
	defer l.stopWg.Done()
	done := make(chan struct{})
	linkUpdateCh := make(chan netlink.LinkUpdate)
	if err := netlink.LinkSubscribe(linkUpdateCh, done); err != nil {
		level.Error(l.Logger).Log("link subscribe: failed to subscribe netlink update events", err)
		close(done)
		close(linkUpdateCh)
		return
	}
	for {
		select {
		case <-l.stop:
			l.closeLinkSubscribe(done, linkUpdateCh)
			return
		case linkUpdateEvent, ok := <-linkUpdateCh:
			if !ok {
				level.Error(l.Logger).Log("link subscribe", "failed to receive link update event")
				l.closeLinkSubscribe(done, linkUpdateCh)
				return
			}
			ifName := linkUpdateEvent.Link.Attrs().Name
			l.mutex.Lock()
			if ch, ok := l.ifaceChannels[ifName]; ok {
				ch <- linkUpdateEvent
			}
			l.mutex.Unlock()
		}
	}
}

func (l *LinkEventHandler) closeLinkSubscribe(done chan struct{}, linkUpdateCh chan netlink.LinkUpdate) {
	close(done)
	// `linkUpdateCh` should be fully read after the `done` close to prevent goroutine leak in `netlink.LinkSubscribe`
	go func() {
		for range linkUpdateCh {
		}
	}()
}
