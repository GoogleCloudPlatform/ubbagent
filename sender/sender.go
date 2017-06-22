package sender

import (
	"ubbagent/pipeline"
	"ubbagent/metrics"
)

// PreparedSend is returned by Sender.Prepare() and is used to execute the actual send.
type PreparedSend interface {
	// Send sends an already-prepared report. This method can still generate an error due to
	// unforeseen transient problems (such as network or persistence problems).
	Send() error
}

// Sender handles sending MetricBatch objects to remote endpoints.
// The Sender interface is split into a prepare step and a send step, similar to a two-phase commit.
// Sending batches to remote endpoints can involve a pre-processing step which might fail. When
// fanning out to multiple endpoints, preprocessing errors can be caught prior to actually sending.
type Sender interface {
	// Sender is a pipeline.Component.
	pipeline.Component

	// Prepare prepares a batch for sending, and returns a Sender used to execute the send.
	// Any failure during the preparation step will be returned as an error.
	Prepare(metrics.MetricBatch) (PreparedSend, error)
}
