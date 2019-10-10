// Copyright © 2017 Heptio
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// workgroup provides a mechanism for controlling the lifetime
// of a set of related goroutines.
package workgroup

import "sync"

// A Group manages a set of goroutines with related lifetimes.
// The zero value for a Group is fully usable without initalisation.
type Group struct {
	fn []func(<-chan struct{}) error
}

// Add adds a function to the Group.
// The function will be exectuted in its own goroutine when Run is called.
// Add must be called before Run.
func (g *Group) Add(fn func(<-chan struct{}) error) {
	g.fn = append(g.fn, fn)
}

// Run exectues each function registered via Add in its own goroutine.
// Run blocks until all functions have returned.
// The first function to return will trigger the closure of the channel
// passed to each function, who should in turn, return.
// The return value from the first function to exit will be returned to
// the caller of Run.
func (g *Group) Run() error {

	// if there are no registered functions, return immediately.
	if len(g.fn) < 1 {
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(len(g.fn))

	stop := make(chan struct{})
	result := make(chan error, len(g.fn))
	for _, fn := range g.fn {
		go func(fn func(<-chan struct{}) error) {
			defer wg.Done()
			result <- fn(stop)
		}(fn)
	}

	defer wg.Wait()
	defer close(stop)
	return <-result
}
