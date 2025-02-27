package metric

import (
	"context"
	"time"

	"github.com/hashicorp/boundary/globals"
	metric "github.com/hashicorp/boundary/internal/daemon/internal/metric"
	"github.com/hashicorp/boundary/internal/gen/controller/servers/services"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	clusterClientSubsystem = "cluster_client"
)

// grpcRequestLatency collects measurements of how long a gRPC
// request between a cluster and its clients takes.
var grpcRequestLatency prometheus.ObserverVec = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: globals.MetricNamespace,
		Subsystem: clusterClientSubsystem,
		Name:      "grpc_request_duration_seconds",
		Help:      "Histogram of latencies for gRPC requests between the cluster and any of its clients.",
		Buckets:   prometheus.DefBuckets,
	},
	metric.ListGrpcLabels,
)

type requestRecorder struct {
	reqLatency prometheus.ObserverVec
	labels     prometheus.Labels

	// measurements
	start time.Time
}

// NewRequestRecorder creates a requestRecorder struct which is used to measure gRPC client request latencies.
// For testing purposes, this method is exported.
func newRequestRecorder(fullMethodName string, reqLatency prometheus.ObserverVec) requestRecorder {
	service, method := metric.SplitMethodName(fullMethodName)
	r := requestRecorder{
		reqLatency: reqLatency,
		labels: prometheus.Labels{
			metric.LabelGrpcMethod:  method,
			metric.LabelGrpcService: service,
		},
		start: time.Now(),
	}

	return r
}

func (r requestRecorder) Record(err error) {
	r.labels[metric.LabelGrpcCode] = metric.StatusFromError(err).Code().String()
	r.reqLatency.With(r.labels).Observe(time.Since(r.start).Seconds())
}

// The expected codes returned by the grpc client calls to cluster services.
var expectedGrpcClientCodes = []codes.Code{
	codes.OK, codes.Canceled, codes.Unknown, codes.InvalidArgument, codes.DeadlineExceeded, codes.NotFound,
	codes.AlreadyExists, codes.PermissionDenied, codes.Unauthenticated, codes.ResourceExhausted,
	codes.FailedPrecondition, codes.Aborted, codes.OutOfRange, codes.Unimplemented, codes.Internal,
	codes.Unavailable, codes.DataLoss,
}

// InstrumentClusterClient wraps a UnaryClientInterceptor and records
// observations for the collectors associated with gRPC connections
// between the cluster and its clients.
func InstrumentClusterClient() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		r := newRequestRecorder(method, grpcRequestLatency)
		err := invoker(ctx, method, req, reply, cc, opts...)
		r.Record(err)
		return err
	}
}

// InitializeClusterClientCollectors registers the cluster client metrics to the
// prometheus register and initializes them to 0 for all possible label
// combinations.
func InitializeClusterClientCollectors(r prometheus.Registerer) {
	metric.InitializeGrpcCollectorsFromPackage(r, grpcRequestLatency, services.File_controller_servers_services_v1_session_service_proto, expectedGrpcClientCodes)
}
