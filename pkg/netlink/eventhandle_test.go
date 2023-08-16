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
	"time"

	"github.com/go-kit/log"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-kni/eapol-operator/internal/logging"
	"github.com/vishvananda/netlink"
)

var _ = Describe("EventHandle", func() {
	var (
		logger log.Logger
	)
	BeforeEach(func() {
		var err error
		logger, err = logging.Init("info")
		Expect(err).NotTo(HaveOccurred())
	})
	Context("Test event handle functions", func() {
		It("Subscribe and unsubscribe netlink update events", func() {
			handler := LinkEventHandler{Logger: logger}
			handler.Start()
			ch := make(chan struct{})
			go func() {
				handler.stopWg.Wait()
				close(ch)
			}()
			ifEventCh := make(chan netlink.LinkUpdate)
			err := handler.Subscribe(ifEventCh, "dummylink")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(handler.ifaceChannels)).To(Equal(1))
			handler.Unsubscribe("dummylink")
			Eventually(func() bool {
				select {
				case <-ifEventCh:
					return false
				default:
					return true
				}
			}, 2*time.Second, 200*time.Millisecond).Should(BeTrue())
			Expect(len(handler.ifaceChannels)).To(Equal(0))
			handler.StopHandler()
			Eventually(func() bool {
				select {
				case <-ch:
					return true
				default:
					return false
				}
			}, 5*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
})
