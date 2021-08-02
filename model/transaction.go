// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package model

import (
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/utility"
)

const (
	transactionProcessorName = "transaction"
	transactionDocType       = "transaction"
	TracesDataset            = "apm"
)

var (
	transactionMetrics         = monitoring.Default.NewRegistry("apm-server.processor.transaction")
	transactionTransformations = monitoring.NewInt(transactionMetrics, "transformations")
	transactionProcessorEntry  = common.MapStr{"name": transactionProcessorName, "event": transactionDocType}
)

type Transaction struct {
	ID       string
	ParentID string
	TraceID  string

	Timestamp time.Time

	Type           string
	Name           string
	Result         string
	Outcome        string
	Duration       float64
	Marks          TransactionMarks
	Message        *Message
	Sampled        bool
	SpanCount      SpanCount
	Page           *Page
	HTTP           *HTTP
	URL            *URL
	Labels         common.MapStr
	Custom         common.MapStr
	UserExperience *UserExperience
	Session        TransactionSession

	Experimental interface{}

	// RepresentativeCount holds the approximate number of
	// transactions that this transaction represents for aggregation.
	//
	// This may be used for scaling metrics; it is not indexed.
	RepresentativeCount float64
}

type TransactionSession struct {
	// ID holds a session ID for grouping a set of related transactions.
	ID string

	// Sequence holds an optional sequence number for a transaction
	// within a session. Sequence is ignored if it is zero or if
	// ID is empty.
	Sequence int
}

type SpanCount struct {
	Dropped *int
	Started *int
}

// fields creates the fields to populate in the top-level "transaction" object field.
func (e *Transaction) fields() common.MapStr {
	var fields mapStr
	fields.set("id", e.ID)
	fields.set("type", e.Type)
	fields.set("duration", utility.MillisAsMicros(e.Duration))
	fields.maybeSetString("name", e.Name)
	fields.maybeSetString("result", e.Result)
	fields.maybeSetMapStr("marks", e.Marks.fields())
	fields.maybeSetMapStr("page", e.Page.Fields())
	fields.maybeSetMapStr("custom", customFields(e.Custom))
	fields.maybeSetMapStr("message", e.Message.Fields())
	fields.maybeSetMapStr("experience", e.UserExperience.Fields())
	if e.SpanCount.Dropped != nil || e.SpanCount.Started != nil {
		spanCount := common.MapStr{}
		if e.SpanCount.Dropped != nil {
			spanCount["dropped"] = *e.SpanCount.Dropped
		}
		if e.SpanCount.Started != nil {
			spanCount["started"] = *e.SpanCount.Started
		}
		fields.set("span_count", spanCount)
	}
	fields.set("sampled", e.Sampled)
	return common.MapStr(fields)
}

func (e *Transaction) toBeatEvent() beat.Event {
	transactionTransformations.Inc()

	fields := mapStr{
		"processor":        transactionProcessorEntry,
		transactionDocType: e.fields(),
	}

	var parent, trace mapStr
	parent.maybeSetString("id", e.ParentID)
	trace.maybeSetString("id", e.TraceID)
	fields.maybeSetMapStr("parent", common.MapStr(parent))
	fields.maybeSetMapStr("trace", common.MapStr(trace))
	fields.maybeSetMapStr("timestamp", utility.TimeAsMicros(e.Timestamp))
	if e.HTTP != nil {
		fields.maybeSetMapStr("http", e.HTTP.transactionTopLevelFields())
	}
	fields.maybeSetMapStr("url", e.URL.Fields())
	fields.maybeSetMapStr("session", e.Session.fields())
	if e.Experimental != nil {
		fields.set("experimental", e.Experimental)
	}
	common.MapStr(fields).Put("event.outcome", e.Outcome)

	return beat.Event{
		Timestamp: e.Timestamp,
		Fields:    common.MapStr(fields),
	}
}

type TransactionMarks map[string]TransactionMark

func (m TransactionMarks) fields() common.MapStr {
	if len(m) == 0 {
		return nil
	}
	out := make(mapStr, len(m))
	for k, v := range m {
		out.maybeSetMapStr(sanitizeLabelKey(k), v.fields())
	}
	return common.MapStr(out)
}

type TransactionMark map[string]float64

func (m TransactionMark) fields() common.MapStr {
	if len(m) == 0 {
		return nil
	}
	out := make(common.MapStr, len(m))
	for k, v := range m {
		out[sanitizeLabelKey(k)] = common.Float(v)
	}
	return out
}

func (s *TransactionSession) fields() common.MapStr {
	if s.ID == "" {
		return nil
	}
	out := common.MapStr{"id": s.ID}
	if s.Sequence > 0 {
		out["sequence"] = s.Sequence
	}
	return out
}
