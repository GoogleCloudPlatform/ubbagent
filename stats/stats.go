package stats

// A StatsRecorder records the result of sending a metrics.MetricBatch to one or more endpoints.
//
// A StatsRecorder expects the following flow:
// 1. The Register method is called immediately prior to performing a send. The method is passed an
//    ExpectedSend instance, which is likely provided by sender.PreparedSend. The agent's
//    aggregator.Aggregator instance calls Register.
// 2. As each handler succeeds or fails in performing its portion of the overall operation, it
//    registers the result using the SendSucceeded and SendFailed methods. The handlers are most
//    likely instances of sender.RetryingSender, wrapping endpoints.
//
// The batchId value should be set to the value of a MetricsBatch.Id. A handler should most likely
// be set to the name of an endpoint handling part of the send operation.
type StatsRecorder interface {
	Register(send ExpectedSend)
	SendSucceeded(batchId string, handler string)
	SendFailed(batchId string, handler string)
}

// An ExpectedSend represents a report that is about to be sent to 1 or more endpoints. ExpectedSend
// provides both an identifier and a list of handlers that will carry out the send operation. Each
// handler is expected to register its ultimate success or failure using the same identifier.
type ExpectedSend interface {
	BatchId() string
	Handlers() []string
}

type noopStatsRecorder struct{}

// NewNoopStatsRecorder returns a StatsRecorder that does nothing.
func NewNoopStatsRecorder() StatsRecorder {
	return &noopStatsRecorder{}
}

func (*noopStatsRecorder) Register(ExpectedSend)        {}
func (*noopStatsRecorder) SendSucceeded(string, string) {}
func (*noopStatsRecorder) SendFailed(string, string)    {}
