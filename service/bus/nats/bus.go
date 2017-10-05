package nats

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/nats-io/go-nats"

	"github.com/weaveworks/flux"
	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/guid"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/service"
	"github.com/weaveworks/flux/service/bus"
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
	methodGitRepoConfig   = ".Platform.GitRepoConfig"
)

var (
	timeout = defaultTimeout
	encoder = nats.EncoderForType(encoderType)
)

type NATS struct {
	url string
	// It's convenient to send (or request) on an encoding connection,
	// since that'll do encoding work for us. When receiving though,
	// we want to decode based on the method as given in the subject,
	// so we use a regular connection and do the decoding ourselves.
	enc     *nats.EncodedConn
	raw     *nats.Conn
	metrics bus.Metrics
}

var _ bus.MessageBus = &NATS{}

func NewMessageBus(url string, metrics bus.Metrics) (*NATS, error) {
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
func (n *NATS) AwaitPresence(instID service.InstanceID, timeout time.Duration) error {
	timer := time.After(timeout)
	attempts := time.NewTicker(presenceTick)
	defer attempts.Stop()

	ctx := context.Background()

	for {
		select {
		case <-attempts.C:
			if err := n.Ping(ctx, instID); err == nil {
				return nil
			}
		case <-timer:
			return remote.UnavailableError(errors.New("presence timeout"))
		}
	}
}

func (n *NATS) Ping(ctx context.Context, instID service.InstanceID) error {
	var response PingResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := n.enc.RequestWithContext(ctx, string(instID)+methodPing, pingReq{}, &response); err != nil {
		return remote.UnavailableError(err)
	}
	return extractError(response.ErrorResponse)
}

// ErrorResponse is for dropping into response structs to carry error
// information over the bus.
//
// The field `ApplicationError` carries either nil (no error), or an
// application-level error. The field `Error`, if non-empty,
// represents any other kind of error.
type ErrorResponse struct {
	ApplicationError *fluxerr.Error `json:",omitempty"`
	Error            string         `json:",omitempty"`
}

type pingReq struct{}

type PingResponse struct {
	ErrorResponse `json:",omitempty`
}

type versionReq struct{}

type VersionResponse struct {
	Result        string
	ErrorResponse `json:",omitempty`
}

type exportReq struct{}

type ExportResponse struct {
	Result        []byte
	ErrorResponse `json:",omitempty`
}

type ListServicesResponse struct {
	Result        []flux.ServiceStatus
	ErrorResponse `json:",omitempty`
}

type ListImagesResponse struct {
	Result        []flux.ImageStatus
	ErrorResponse `json:",omitempty`
}

type UpdateManifestsResponse struct {
	Result        job.ID
	ErrorResponse `json:",omitempty`
}

type syncReq struct{}
type SyncNotifyResponse struct {
	ErrorResponse `json:",omitempty`
}

// JobStatusResponse has status decomposed into it, so that we can transfer the
// error as an ErrorResponse to avoid marshalling issues.
type JobStatusResponse struct {
	Result        job.Status
	ErrorResponse `json:",omitempty`
}

type SyncStatusResponse struct {
	Result        []string
	ErrorResponse `json:",omitempty`
}

type GitRepoConfigResponse struct {
	Result        flux.GitConfig
	ErrorResponse `json:",omitempty`
}

func extractError(resp ErrorResponse) error {
	var err error
	if resp.Error != "" {
		err = errors.New(resp.Error)
	}
	if resp.ApplicationError != nil {
		err = resp.ApplicationError
	}
	return err
}

func makeErrorResponse(err error) ErrorResponse {
	var resp ErrorResponse
	if err != nil {
		if err, ok := err.(*fluxerr.Error); ok {
			resp.ApplicationError = err
			return resp
		}
		resp.Error = err.Error()
	}
	return resp
}

// natsPlatform collects the things you need to make a request via NATS
// together, and implements remote.Platform using that mechanism.
type natsPlatform struct {
	conn     *nats.EncodedConn
	instance string
}

func (r *natsPlatform) Ping(ctx context.Context) error {
	var response PingResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodPing, pingReq{}, &response); err != nil {
		return remote.UnavailableError(err)
	}
	return extractError(response.ErrorResponse)
}

func (r *natsPlatform) Version(ctx context.Context) (string, error) {
	var response VersionResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodVersion, versionReq{}, &response); err != nil {
		return response.Result, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) Export(ctx context.Context) ([]byte, error) {
	var response ExportResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodExport, exportReq{}, &response); err != nil {
		return response.Result, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) ListServices(ctx context.Context, namespace string) ([]flux.ServiceStatus, error) {
	var response ListServicesResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodListServices, namespace, &response); err != nil {
		return response.Result, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) ListImages(ctx context.Context, spec update.ServiceSpec) ([]flux.ImageStatus, error) {
	var response ListImagesResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodListImages, spec, &response); err != nil {
		return response.Result, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) UpdateManifests(ctx context.Context, u update.Spec) (job.ID, error) {
	var response UpdateManifestsResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodUpdateManifests, u, &response); err != nil {
		return response.Result, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) SyncNotify(ctx context.Context) error {
	var response SyncNotifyResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodSyncNotify, syncReq{}, &response); err != nil {
		return remote.UnavailableError(err)
	}
	return extractError(response.ErrorResponse)
}

func (r *natsPlatform) JobStatus(ctx context.Context, jobID job.ID) (job.Status, error) {
	var response JobStatusResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodJobStatus, jobID, &response); err != nil {
		return response.Result, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) SyncStatus(ctx context.Context, ref string) ([]string, error) {
	var response SyncStatusResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodSyncStatus, ref, &response); err != nil {
		return nil, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) GitRepoConfig(ctx context.Context, regenerate bool) (flux.GitConfig, error) {
	var response GitRepoConfigResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodGitRepoConfig, regenerate, &response); err != nil {
		return response.Result, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

// --- end Platform implementation

// Connect returns a remote.Platform implementation that can be used
// to talk to a particular instance.
func (n *NATS) Connect(instID service.InstanceID) (remote.Platform, error) {
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
func (n *NATS) Subscribe(ctx context.Context, instID service.InstanceID, platform remote.Platform, done chan<- error) {
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

	go func() {
		forceReconnect := time.NewTimer(maxAge)
		defer forceReconnect.Stop()
		for {
			select {
			case <-ctx.Done():
				sub.Unsubscribe()
				close(requests)
				done <- ctx.Err()
				return
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
				go n.processRequest(ctx, request, instID, platform, myID, errc)
			case <-forceReconnect.C:
				sub.Unsubscribe()
				close(requests)
				done <- nil
				return
			}
		}
	}()
}

func (n *NATS) processRequest(ctx context.Context, request *nats.Msg, instID service.InstanceID, platform remote.Platform, myID string, errc chan<- error) {
	var err error
	switch {
	case strings.HasSuffix(request.Subject, methodKick):
		id := string(request.Data)
		if id != myID {
			n.metrics.IncrKicks(instID)
			err = remote.FatalError{errors.New("Kicked by new subscriber " + id)}
		}

	case strings.HasSuffix(request.Subject, methodPing):
		var p pingReq
		err = encoder.Decode(request.Subject, request.Data, &p)
		if err == nil {
			err = platform.Ping(ctx)
		}
		n.enc.Publish(request.Reply, PingResponse{makeErrorResponse(err)})

	case strings.HasSuffix(request.Subject, methodVersion):
		var vsn string
		vsn, err = platform.Version(ctx)
		n.enc.Publish(request.Reply, VersionResponse{vsn, makeErrorResponse(err)})

	case strings.HasSuffix(request.Subject, methodExport):
		var (
			req   exportReq
			bytes []byte
		)
		err = encoder.Decode(request.Subject, request.Data, &req)
		if err == nil {
			bytes, err = platform.Export(ctx)
		}
		n.enc.Publish(request.Reply, ExportResponse{bytes, makeErrorResponse(err)})

	case strings.HasSuffix(request.Subject, methodListServices):
		var (
			namespace string
			res       []flux.ServiceStatus
		)
		err = encoder.Decode(request.Subject, request.Data, &namespace)
		if err == nil {
			res, err = platform.ListServices(ctx, namespace)
		}
		n.enc.Publish(request.Reply, ListServicesResponse{res, makeErrorResponse(err)})

	case strings.HasSuffix(request.Subject, methodListImages):
		var (
			req update.ServiceSpec
			res []flux.ImageStatus
		)
		err = encoder.Decode(request.Subject, request.Data, &req)
		if err == nil {
			res, err = platform.ListImages(ctx, req)
		}
		n.enc.Publish(request.Reply, ListImagesResponse{res, makeErrorResponse(err)})

	case strings.HasSuffix(request.Subject, methodUpdateManifests):
		var (
			req update.Spec
			res job.ID
		)
		err = encoder.Decode(request.Subject, request.Data, &req)
		if err == nil {
			res, err = platform.UpdateManifests(ctx, req)
		}
		n.enc.Publish(request.Reply, UpdateManifestsResponse{res, makeErrorResponse(err)})

	case strings.HasSuffix(request.Subject, methodSyncNotify):
		var p syncReq
		err = encoder.Decode(request.Subject, request.Data, &p)
		if err == nil {
			err = platform.SyncNotify(ctx)
		}
		n.enc.Publish(request.Reply, SyncNotifyResponse{makeErrorResponse(err)})

	case strings.HasSuffix(request.Subject, methodJobStatus):
		var (
			req job.ID
			res job.Status
		)
		err = encoder.Decode(request.Subject, request.Data, &req)
		if err == nil {
			res, err = platform.JobStatus(ctx, req)
		}
		n.enc.Publish(request.Reply, JobStatusResponse{
			Result:        res,
			ErrorResponse: makeErrorResponse(err),
		})

	case strings.HasSuffix(request.Subject, methodSyncStatus):
		var (
			req string
			res []string
		)
		err = encoder.Decode(request.Subject, request.Data, &req)
		if err == nil {
			res, err = platform.SyncStatus(ctx, req)
		}
		n.enc.Publish(request.Reply, SyncStatusResponse{res, makeErrorResponse(err)})

	case strings.HasSuffix(request.Subject, methodGitRepoConfig):
		var (
			req bool
			res flux.GitConfig
		)
		err = encoder.Decode(request.Subject, request.Data, &req)
		if err == nil {
			res, err = platform.GitRepoConfig(ctx, req)
		}
		n.enc.Publish(request.Reply, GitRepoConfigResponse{res, makeErrorResponse(err)})

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
