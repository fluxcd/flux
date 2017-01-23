package nats

import (
	"errors"
	"net/rpc"
	"strings"
	"time"

	"github.com/nats-io/nats"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/guid"
	"github.com/weaveworks/flux/platform"
	fluxrpc "github.com/weaveworks/flux/platform/rpc"
)

const (
	timeout      = 5 * time.Second
	applyTimeout = 20 * time.Minute
	presenceTick = 50 * time.Millisecond
	encoderType  = nats.JSON_ENCODER

	methodKick         = ".Platform.Kick"
	methodPing         = ".Platform.Ping"
	methodVersion      = ".Platform.Version"
	methodAllServices  = ".Platform.AllServices"
	methodSomeServices = ".Platform.SomeServices"
	methodApply        = ".Platform.Apply"
)

type NATS struct {
	url string
	// It's convenient to send (or request) on an encoding connection,
	// since that'll do encoding work for us. When receiving though,
	// we want to decode based on the method as given in the subject,
	// so we use a regular connection and do the decoding ourselves.
	enc     *nats.EncodedConn
	raw     *nats.Conn
	metrics platform.BusMetrics
}

var _ platform.MessageBus = &NATS{}

func NewMessageBus(url string, metrics platform.BusMetrics) (*NATS, error) {
	conn, err := nats.Connect(url)
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
			return errors.New("presence timeout")
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

type AllServicesResponse struct {
	Services []platform.Service
	ErrorResponse
}

type SomeServicesResponse struct {
	Services []platform.Service
	ErrorResponse
}

type ApplyResponse struct {
	Result fluxrpc.ApplyResult
	ErrorResponse
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

func extractError(resp ErrorResponse) error {
	if resp.Error != "" {
		if resp.Fatal {
			return platform.FatalError{errors.New(resp.Error)}
		}
		return rpc.ServerError(resp.Error)
	}
	return nil
}

func makeErrorResponse(err error) (resp ErrorResponse) {
	if err == nil {
		return resp
	}
	if _, ok := err.(platform.FatalError); ok {
		resp.Fatal = true
	}
	resp.Error = err.Error()
	return resp
}

// natsPlatform collects the things you need to make a request via NATS
// together, and implements platform.Platform using that mechanism.
type natsPlatform struct {
	conn     *nats.EncodedConn
	instance string
}

func (r *natsPlatform) AllServices(ns string, ig flux.ServiceIDSet) ([]platform.Service, error) {
	var response AllServicesResponse
	if err := r.conn.Request(r.instance+methodAllServices, fluxrpc.AllServicesRequest{ns, ig}, &response, timeout); err != nil {
		return nil, err
	}
	return response.Services, extractError(response.ErrorResponse)
}

func (r *natsPlatform) SomeServices(incl []flux.ServiceID) ([]platform.Service, error) {
	var response SomeServicesResponse
	if err := r.conn.Request(r.instance+methodSomeServices, incl, &response, timeout); err != nil {
		return nil, err
	}
	return response.Services, extractError(response.ErrorResponse)
}

// Call Apply on the remote platform. Note that we use a much longer
// timeout, because for now at least, Applys can take an arbitrary
// amount of time, and we don't want to return an error if it's simply
// taking a while. The downside is that if the platform is actually
// not present, this won't return at all. This is somewhat mitigated
// because applys are done after other RPCs which have the normal
// timeout, but better would be to split Applys into RPCs which can
// each have a short timeout.
func (r *natsPlatform) Apply(specs []platform.ServiceDefinition) error {
	var response ApplyResponse
	if err := r.conn.Request(r.instance+methodApply, specs, &response, applyTimeout); err != nil {
		return err
	}
	if len(response.Result) > 0 {
		errs := platform.ApplyError{}
		for s, e := range response.Result {
			errs[s] = errors.New(e)
		}
		return errs
	}
	return extractError(response.ErrorResponse)
}

func (r *natsPlatform) Ping() error {
	var response PingResponse
	if err := r.conn.Request(r.instance+methodPing, ping{}, &response, timeout); err != nil {
		return err
	}
	return extractError(response.ErrorResponse)
}

func (r *natsPlatform) Version() (string, error) {
	var response VersionResponse
	if err := r.conn.Request(r.instance+methodVersion, version{}, &response, timeout); err != nil {
		return "", err
	}
	return response.Version, extractError(response.ErrorResponse)
}

// Connect returns a platform.Platform implementation that can be used
// to talk to a particular instance.
func (n *NATS) Connect(instID flux.InstanceID) (platform.Platform, error) {
	return &natsPlatform{
		conn:     n.enc,
		instance: string(instID),
	}, nil
}

// Subscribe registers a remote platform.Platform implementation as
// the daemon for an instance (identified by instID). Any
// platform.FatalError returned when processing requests will result
// in the platform being deregistered, with the error put on the
// channel `done`.
func (n *NATS) Subscribe(instID flux.InstanceID, remote platform.Platform, done chan<- error) {
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

	go func() {
		var err error
		for request := range requests {
			switch {
			case strings.HasSuffix(request.Subject, methodKick):
				id := string(request.Data)
				if id != myID {
					n.metrics.IncrKicks(instID)
					err = platform.FatalError{errors.New("Kicked by new subscriber " + id)}
				}
			case strings.HasSuffix(request.Subject, methodPing):
				var p ping
				err = encoder.Decode(request.Subject, request.Data, &p)
				if err == nil {
					err = remote.Ping()
				}
				n.enc.Publish(request.Reply, PingResponse{makeErrorResponse(err)})
			case strings.HasSuffix(request.Subject, methodVersion):
				var vsn string
				vsn, err = remote.Version()
				n.enc.Publish(request.Reply, VersionResponse{vsn, makeErrorResponse(err)})
			case strings.HasSuffix(request.Subject, methodAllServices):
				var (
					req fluxrpc.AllServicesRequest
					res []platform.Service
				)
				err = encoder.Decode(request.Subject, request.Data, &req)
				if err == nil {
					res, err = remote.AllServices(req.MaybeNamespace, req.Ignored)
				}
				n.enc.Publish(request.Reply, AllServicesResponse{res, makeErrorResponse(err)})
			case strings.HasSuffix(request.Subject, methodSomeServices):
				var (
					req []flux.ServiceID
					res []platform.Service
				)
				err = encoder.Decode(request.Subject, request.Data, &req)
				if err == nil {
					res, err = remote.SomeServices(req)
				}
				n.enc.Publish(request.Reply, SomeServicesResponse{res, makeErrorResponse(err)})
			case strings.HasSuffix(request.Subject, methodApply):
				var (
					req []platform.ServiceDefinition
				)
				err = encoder.Decode(request.Subject, request.Data, &req)
				if err == nil {
					err = remote.Apply(req)
				}
				response := ApplyResponse{}
				switch applyErr := err.(type) {
				case platform.ApplyError:
					result := fluxrpc.ApplyResult{}
					for s, e := range applyErr {
						result[s] = e.Error()
					}
					response.Result = result
				default:
					response.ErrorResponse = makeErrorResponse(err)
				}
				n.enc.Publish(request.Reply, response)
			default:
				err = errors.New("unknown message: " + request.Subject)
			}
			if _, ok := err.(platform.FatalError); ok && err != nil {
				sub.Unsubscribe()
				close(requests)
				done <- err
				return
			}
		}
	}()
}
