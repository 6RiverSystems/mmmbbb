// Copyright (c) 2024 6 River Systems
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package internal

import (
	"net"
	"os"
	"strconv"
	"sync"

	"go.6river.tech/mmmbbb/logging"
)

// randomizedPorts is, functionally, a `map[int]int`, and is mostly for testing,
// or other cases where we want to pick a random free TCP port for listening,
// instead of a pre-determined one
var randomizedPorts *sync.Map

func EnableRandomPorts() {
	if randomizedPorts == nil {
		randomizedPorts = new(sync.Map)
	}
}

func getRandomPort(port int) int {
	rPort, ok := randomizedPorts.Load(port)
	if !ok {
		// make & destroy a listener to select a random port, hopefully it will
		// still be available when the real app code needs to use it
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			panic(err)
		}
		defer listener.Close()
		rPort = listener.Addr().(*net.TCPAddr).Port
		var loaded bool
		rPort, loaded = randomizedPorts.LoadOrStore(port, rPort)
		if !loaded {
			logging.GetLogger("server").Info().
				Int("port", port).
				Int("randomPort", rPort.(int)).
				Msg("Using randomized port")
		}
	}
	return rPort.(int)
}

func ResolvePort(defaultBasePort, offset int) int {
	if randomizedPorts != nil {
		return getRandomPort(defaultBasePort + offset)
	} else if port := os.Getenv("PORT"); port != "" {
		listenPort, err := strconv.ParseInt(port, 10, 16)
		if err != nil {
			panic(err)
		}
		return int(listenPort) + offset
	} else {
		return defaultBasePort + offset
	}
}
