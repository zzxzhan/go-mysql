/*
	Copyright (c) 2014-2015, Percona LLC and/or its affiliates. All rights reserved.

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU Affero General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU Affero General Public License for more details.

	You should have received a copy of the GNU Affero General Public License
	along with this program.  If not, see <http://www.gnu.org/licenses/>
*/

package event

import (
	"time"

	"github.com/percona/go-mysql/log"
)

// A Result contains a global class and per-ID classes with finalized metric
// statistics. The classes are keyed on class ID.
type Result struct {
	Global *Class            // all classes
	Class  map[string]*Class // keyed on class ID
	Error  string
}

// An Aggregator groups events by class ID. When there are no more events,
// a call to Finalize computes all metric statistics and returns a Result.
type Aggregator struct {
	samples     bool
	utcOffset   time.Duration
	outlierTime float64
	// --
	global  *Class
	classes map[string]*Class
}

// NewAggregator returns a new Aggregator.
func NewAggregator(samples bool, utcOffset time.Duration, outlierTime float64) *Aggregator {
	a := &Aggregator{
		samples:     samples,
		utcOffset:   utcOffset,
		outlierTime: outlierTime,
		// --
		global:  NewClass("", "", false),
		classes: make(map[string]*Class),
	}
	return a
}

// AddEvent adds the event to the aggregator, automatically creating new classes
// as needed.
func (a *Aggregator) AddEvent(event *log.Event, id, fingerprint string) {
	outlier := false
	if a.outlierTime > 0 && event.TimeMetrics["Query_time"] > a.outlierTime {
		outlier = true
	}

	a.global.AddEvent(event, outlier)

	class, ok := a.classes[id]
	if !ok {
		class = NewClass(id, fingerprint, a.samples)
		a.classes[id] = class
	}
	class.AddEvent(event, outlier)
}

// Finalize calculates all metric statistics and returns a Result.
// Call this function when done adding events to the aggregator.
func (a *Aggregator) Finalize(rate uint) Result {
	if rate == 0 {
		rate = 1
	}
	a.global.Finalize(rate)
	a.global.UniqueQueries = uint(len(a.classes))
	for _, class := range a.classes {
		class.Finalize(rate)
		class.UniqueQueries = 1
		if class.Sample != nil && class.Sample.Ts != "" {
			if t, err := time.Parse("060102 15:04:05", class.Sample.Ts); err != nil {
				class.Sample.Ts = ""
			} else {
				class.Sample.Ts = t.Add(a.utcOffset).Format("2006-01-02 15:04:05")
			}
		}
	}
	return Result{
		Global: a.global,
		Class:  a.classes,
	}
}
