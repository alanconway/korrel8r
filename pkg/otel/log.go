// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

package otel

import "time"

// OTELLog represents the OTEL log data model defined at:
// https://opentelemetry.io/docs/specs/otel/logs/data-model
type Log struct {
	Timestamp time.Time `json:"timestamp"`    // ← Loki timestamp
	Body      any       `json:"body"`         // ← Log line or structure
	Severity  string    `json:"severityText"` // ← Can become a label
	// FIXME combine resource & attributes? Check log OTEL model plan.
	Attributes map[string]any `json:"attributes,omitempty"` // Attributes of this log record.
}

// FIXME use for otel and api logs.

// FIXME consider these:
// ObservedTimestamp	Time when the event was observed.
// TraceId	Request trace id.
// SpanId	Request span id.
// TraceFlags	W3C trace flag.
// SeverityText	The severity text (also known as log level).
// SeverityNumber	Numerical value of the severity.
// Body	The body of the log record.
// Resource	Describes the source of the log.
// InstrumentationScope	Describes the scope that emitted the log.
// Attributes	Additional information about the event.
// EventName	Name that identifies the class / type of event.
