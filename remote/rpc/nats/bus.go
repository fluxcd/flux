package nats

import (
	"errors"
	"net/rpc"
	"strings"
	"time"

	"github.com/nats-io/nats"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/guid"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
)

const (
	// We give subscriptions an age limit, because if we have very
	// long-lived connections we don't get fine-enough-grained usage
	// metrics
	maxAge         = 2 * time.Hour
	defaultTimeout = 10 * time.Second
	presenceTick   = 50 * time.Millisecond
	encoderType    = nats.JSON_ENCODER

	methodKick            = ".Platform.Kick"
	methodPing            = ".Platform.Ping"
	methodVersion         = ".Platform.Version"
	methodExport          = ".Platform.Export"
	methodListServices    = ".Platform.ListServices"
	methodListImages      = ".Platform.ListImages"
	methodSyncNotify      = ".Platform.SyncNotify"
	methodJobStatus       = ".Platform.JobStatus"
	methodSyncStatus      = ".Platform.SyncStatus"
	methodUpdateManifests = ".Platform.UpdateManifests"
)

var timeout = defaultTimeout

type NATS struct {
	url string
	// It's convenient to send (or request) on an encoding connection,
	// since that'll do encoding work for us. When receiving though,
	// we want to decode based on the method as given in the subject,
	// so we use a regular connection and do the decoding ourselves.
	enc     *nats.EncodedConn
	raw     *nats.Conn
	metrics remote.BusMetrics
}

var _ remote.MessageBus = &NATS{}

func NewMessageBus(url string, metrics remote.BusMetrics) (*NATS, error) {
	conn, err := nats.Connect(url, nats.MaxReconnects(-1))
	if err != nil {
		return nil, err
	}
	encConn, err := nats.NewEncodedConn(conn, encoderType)
	if err != nil {
		return nil, err
	}
	return &NATS{
		url:     url,
		raw:     conn,
		enc:     encConn,
		metrics: metrics,
	}, nil
}

// Wait up to `timeout` for a particular instance to connect. Mostly
// useful for synchronising during testing.
func (n *NATS) AwaitPresence(instID flux.InstanceID, timeout time.Duration) error {
	timer := time.After(timeout)
	attempts := time.NewTicker(presenceTick)
	defer attempts.Stop()

	for {
		select {
		case <-attempts.C:
			if err := n.Ping(instID); err == nil {
				return nil
			}
		case <-timer:
			return remote.UnavailableError(errors.New("presence timeout"))
		}
	}
}

func (n *NATS) Ping(instID flux.InstanceID) error {
	var response PingResponse
	err := n.enc.Request(string(instID)+methodPing, ping{}, &response, timeout)
	if err == nil {
		err = extractError(response.ErrorResponse)
	}
	return err
}

// ErrorResponse is for dropping into responses so they have
// appropriate fields. The field `Error` carries either an empty
// string (no error), or the error message to be reconstituted as an
// error). The field `Fatal` indicates that the error resulted in the
// connection to the daemon being torn down.
type ErrorResponse struct {
	Error string
	Fatal bool
}

type ping struct{}

type PingResponse struct {
	ErrorResponse
}

type version struct{}

type VersionResponse struct {
	Version string
	ErrorResponse
}

type export struct{}

type ExportResponse struct {
	Config []byte
	ErrorResponse
}

type ListServicesResponse struct {
	Result []flux.ServiceStatus
	ErrorResponse
}

type ListImagesResponse struct {
	Result []flux.ImageStatus
	ErrorResponse
}

type UpdateManifestsResponse struct {
	Result job.ID
	ErrorResponse
}

type sync struct{}
type SyncNotifyResponse struct {
	ErrorResponse
}

// JobStatusResponse has status decomposed into it, so that we can transfer the
// error as an ErrorResponse to avoid marshalling issues.
type JobStatusResponse struct {
	StatusResult interface{}
	StatusError  ErrorResponse
	StatusString job.StatusString
	ErrorResponse
}

type SyncStatusResponse struct {
	Result []string
	ErrorResponse
}

func extractError(resp ErrorResponse) error {
	if resp.Error != "" {
		if resp.Fatal {
			return remote.FatalError{errors.New(resp.Error)}
		}
		return rpc.ServerError(resp.Error)
	}
	return nil
}

func makeErrorResponse(err error) (resp ErrorResponse) {
	if err == nil {
		return resp
	}
	if _, ok := err.(remote.FatalError); ok {
		resp.Fatal = true
	}
	resp.Error = err.Error()
	return resp
}

// natsPlatform collects the things you need to make a request via NATS
// together, and implements remote.Platform using that mechanism.
type natsPlatform struct {
	conn     *nats.EncodedConn
	instance string
}

func (r *natsPlatform) Ping() error {
	var response PingResponse
	if err := r.conn.Request(r.instance+methodPing, ping{}, &response, timeout); err != nil {
		if err == nats.ErrTimeout {
			err = remote.UnavailableError(err)
		}
		return err
	}
	return extractError(response.ErrorResponse)
}

func (r *natsPlatform) Version() (string, error) {
	var response VersionResponse
	if err := r.conn.Request(r.instance+methodVersion, version{}, &response, timeout); err != nil {
		if err == nats.ErrTimeout {
			err = remote.UnavailableError(err)
		}
		return "", err
	}
	return response.Version, extractError(response.ErrorResponse)
}

func (r *natsPlatform) Export() ([]byte, error) {
	var response ExportResponse
	if err := r.conn.Request(r.instance+methodExport, export{}, &response, timeout); err != nil {
		if err == nats.ErrTimeout {
			err = remote.UnavailableError(err)
		}
		return nil, err
	}
	return response.Config, extractError(response.ErrorResponse)
}

func (r *natsPlatform) ListServices(namespace string) ([]flux.ServiceStatus, error) {
	var response ListServicesResponse
	if err := r.conn.Request(r.instance+methodListServices, namespace, &response, timeout); err != nil {
		if err == nats.ErrTimeout {
			err = remote.UnavailableError(err)
		}
		return nil, err
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) ListImages(spec update.ServiceSpec) ([]flux.ImageStatus, error) {
	var response ListImagesResponse
	if err := r.conn.Request(r.instance+methodListImages, spec, &response, timeout); err != nil {
		if err == nats.ErrTimeout {
			err = remote.UnavailableError(err)
		}
		return nil, err
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) UpdateManifests(u update.Spec) (job.ID, error) {
	var response UpdateManifestsResponse
	if err := r.conn.Request(r.instance+methodUpdateManifests, u, &response, timeout); err != nil {
		if err == nats.ErrTimeout {
			err = remote.UnavailableError(err)
		}
		return response.Result, err
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) SyncNotify() error {
	var response SyncNotifyResponse
	if err := r.conn.Request(r.instance+methodSyncNotify, sync{}, &response, timeout); err != nil {
		if err == nats.ErrTimeout {
			err = remote.UnavailableError(err)
		}
		return err
	}
	return extractError(response.ErrorResponse)
}

func (r *natsPlatform) JobStatus(jobID job.ID) (job.Status, error) {
	var response JobStatusResponse
	if err := r.conn.Request(r.instance+methodJobStatus, jobID, &response, timeout); err != nil {
		if err == nats.ErrTimeout {
			err = remote.UnavailableError(err)
		}
		return job.Status{}, err
	}
	return job.Status{
		Result:       response.StatusResult,
		Error:        extractError(response.StatusError),
		StatusString: response.StatusString,
	}, extractError(response.ErrorResponse)
}

func (r *natsPlatform) SyncStatus(ref string) ([]string, error) {
	var response SyncStatusResponse
	if err := r.conn.Request(r.instance+methodSyncStatus, ref, &response, timeout); err != nil {
		if err == nats.ErrTimeout {
			err = remote.UnavailableError(err)
		}
		return nil, err
	}
	return response.Result, extractError(response.ErrorResponse)
}

// --- end Platform implementation

// Connect returns a remote.Platform implementation that can be used
// to talk to a particular instance.
func (n *NATS) Connect(instID flux.InstanceID) (remote.Platform, error) {
	return &natsPlatform{
		conn:     n.enc,
		instance: string(instID),
	}, nil
}

// Subscribe registers a remote remote.Platform implementation as
// the daemon for an instance (identified by instID). Any
// remote.FatalError returned when processing requests will result
// in the platform being deregistered, with the error put on the
// channel `done`.
func (n *NATS) Subscribe(instID flux.InstanceID, platform remote.Platform, done chan<- error) {
	encoder := nats.EncoderForType(encoderType)

	requests := make(chan *nats.Msg)
	sub, err := n.raw.ChanSubscribe(string(instID)+".Platform.>", requests)
	if err != nil {
		done <- err
		return
	}

	// It's possible that more than one connection for a particular
	// instance will arrive at the service. To prevent confusion, when
	// a subscription arrives, it sends a "kick" message with a unique
	// ID (so it can recognise its own kick message). Any other
	// subscription for the instance _should_ then exit upon receipt
	// of the kick.
	myID := guid.New()
	n.raw.Publish(string(instID)+methodKick, []byte(myID))

	errc := make(chan error)

	processRequest := func(request *nats.Msg) {
		var err error
		switch {
		case strings.HasSuffix(request.Subject, methodKick):
			id := string(request.Data)
			if id != myID {
				n.metrics.IncrKicks(instID)
				err = remote.FatalError{errors.New("Kicked by new subscriber " + id)}
			}

		case strings.HasSuffix(request.Subject, methodPing):
			var p ping
			err = encoder.Decode(request.Subject, request.Data, &p)
			if err == nil {
				err = platform.Ping()
			}
			n.enc.Publish(request.Reply, PingResponse{makeErrorResponse(err)})

		case strings.HasSuffix(request.Subject, methodVersion):
			var vsn string
			vsn, err = platform.Version()
			n.enc.Publish(request.Reply, VersionResponse{vsn, makeErrorResponse(err)})

		case strings.HasSuffix(request.Subject, methodExport):
			var (
				req   export
				bytes []byte
			)
			err = encoder.Decode(request.Subject, request.Data, &req)
			if err == nil {
				bytes, err = platform.Export()
			}
			n.enc.Publish(request.Reply, ExportResponse{bytes, makeErrorResponse(err)})

		case strings.HasSuffix(request.Subject, methodListServices):
			var (
				namespace string
				res       []flux.ServiceStatus
			)
			err = encoder.Decode(request.Subject, request.Data, &namespace)
			if err == nil {
				res, err = platform.ListServices(namespace)
			}
			n.enc.Publish(request.Reply, ListServicesResponse{res, makeErrorResponse(err)})

		case strings.HasSuffix(request.Subject, methodListImages):
			var (
				req update.ServiceSpec
				res []flux.ImageStatus
			)
			err = encoder.Decode(request.Subject, request.Data, &req)
			if err == nil {
				res, err = platform.ListImages(req)
			}
			n.enc.Publish(request.Reply, ListImagesResponse{res, makeErrorResponse(err)})

		case strings.HasSuffix(request.Subject, methodUpdateManifests):
			var (
				req update.Spec
				res job.ID
			)
			err = encoder.Decode(request.Subject, request.Data, &req)
			if err == nil {
				res, err = platform.UpdateManifests(req)
			}
			n.enc.Publish(request.Reply, UpdateManifestsResponse{res, makeErrorResponse(err)})

		case strings.HasSuffix(request.Subject, methodSyncNotify):
			err = platform.SyncNotify()
			n.enc.Publish(request.Reply, SyncNotifyResponse{makeErrorResponse(err)})

		case strings.HasSuffix(request.Subject, methodJobStatus):
			var (
				req job.ID
				res job.Status
			)
			err = encoder.Decode(request.Subject, request.Data, &req)
			if err == nil {
				res, err = platform.JobStatus(req)
			}
			n.enc.Publish(request.Reply, JobStatusResponse{
				StatusResult:  res.Result,
				StatusError:   makeErrorResponse(res.Error),
				StatusString:  res.StatusString,
				ErrorResponse: makeErrorResponse(err),
			})

		case strings.HasSuffix(request.Subject, methodSyncStatus):
			var (
				req string
				res []string
			)
			err = encoder.Decode(request.Subject, request.Data, &req)
			if err == nil {
				res, err = platform.SyncStatus(req)
			}
			n.enc.Publish(request.Reply, SyncStatusResponse{res, makeErrorResponse(err)})

		default:
			err = errors.New("unknown message: " + request.Subject)
		}
		if _, ok := err.(remote.FatalError); ok && err != nil {
			select {
			case errc <- err:
			default:
				// If the error channel is closed, it means that a
				// different RPC goroutine had a fatal error that
				// triggered the clean up and return of the parent
				// goroutine. It is likely that the error we have
				// encountered is due to the closure of the RPC
				// client whilst our request was still in progress
				// - don't panic.
			}
		}
	}

	go func() {
		forceReconnect := time.NewTimer(maxAge)
		defer forceReconnect.Stop()
		for {
			select {
			// If both an error and a request are available, the runtime may
			// chose (by uniform pseudo-random selection) to process the
			// request first. This may seem like a problem, but even if we were
			// guaranteed to prefer the error channel there would still be a
			// race between selecting a request here and a failing goroutine
			// putting an error into the channel - it's an unavoidable
			// consequence of asynchronous request handling. The error will get
			// selected and handled soon enough.
			case err := <-errc:
				sub.Unsubscribe()
				close(requests)
				done <- err
				return
			case request := <-requests:
				// Some of these operations (Apply in particular) can block for a long time;
				// dispatch in a goroutine and deliver any errors back to us so that we can
				// clean up on any hard failures.
				go processRequest(request)
			case <-forceReconnect.C:
				sub.Unsubscribe()
				close(requests)
				done <- nil
				return
			}
		}
	}()
}
