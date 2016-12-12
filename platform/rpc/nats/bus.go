package nats

import (
	"errors"
	"net/rpc"
	"strings"
	"time"

	"github.com/nats-io/nats"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/platform"
	fluxrpc "github.com/weaveworks/flux/platform/rpc"
)

const (
	timeout        = 5 * time.Second
	regradeTimeout = 20 * time.Minute
	presenceTick   = 50 * time.Millisecond
	encoderType    = nats.JSON_ENCODER

	methodPing         = ".Platform.Ping"
	methodAllServices  = ".Platform.AllServices"
	methodSomeServices = ".Platform.SomeServices"
	methodRegrade      = ".Platform.Regrade"
)

type NATS struct {
	url string
	// It's convenient to send (or request) on an encoding connection,
	// since that'll do encoding work for us. When receiving though,
	// we want to decode based on the method as given in the subject,
	// so we use a regular connection and do the decoding ourselves.
	snd *nats.EncodedConn
	rcv *nats.Conn
}

var _ platform.MessageBus = &NATS{}

func NewMessageBus(url string) (*NATS, error) {
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	encConn, err := nats.NewEncodedConn(conn, encoderType)
	if err != nil {
		return nil, err
	}
	return &NATS{
		url: url,
		rcv: conn,
		snd: encConn,
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
	err := n.snd.Request(string(instID)+methodPing, ping{}, &response, timeout)
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

type RegradeResponse struct {
	Result fluxrpc.RegradeResult
	ErrorResponse
}

type ping struct{}

type PingResponse struct {
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

// requester just collect the things you need to make a request
// together
type requester struct {
	conn     *nats.EncodedConn
	instance string
}

func (r *requester) AllServices(ns string, ig flux.ServiceIDSet) ([]platform.Service, error) {
	var response AllServicesResponse
	if err := r.conn.Request(r.instance+methodAllServices, fluxrpc.AllServicesRequest{ns, ig}, &response, timeout); err != nil {
		return nil, err
	}
	return response.Services, extractError(response.ErrorResponse)
}

func (r *requester) SomeServices(incl []flux.ServiceID) ([]platform.Service, error) {
	var response SomeServicesResponse
	if err := r.conn.Request(r.instance+methodSomeServices, incl, &response, timeout); err != nil {
		return nil, err
	}
	return response.Services, extractError(response.ErrorResponse)
}

// Call Regrade on the remote platform. Note that we use a much longer
// timeout, because for now at least, Regrades can take an arbitrary
// amount of time, and we don't want to return an error if it's simply
// taking a while. The downside is that if the platform is actually
// not present, this won't return at all. This is somewhat mitigated
// because regrades are done after other RPCs which have the normal
// timeout, but better would be to split Regrades into RPCs which can
// each have a short timeout.
func (r *requester) Regrade(specs []platform.RegradeSpec) error {
	var response RegradeResponse
	if err := r.conn.Request(r.instance+methodRegrade, specs, &response, regradeTimeout); err != nil {
		return err
	}
	if len(response.Result) > 0 {
		errs := platform.RegradeError{}
		for s, e := range response.Result {
			errs[s] = errors.New(e)
		}
		return errs
	}
	return extractError(response.ErrorResponse)
}

func (r *requester) Ping() error {
	var response PingResponse
	if err := r.conn.Request(r.instance+methodPing, ping{}, &response, timeout); err != nil {
		return err
	}
	return extractError(response.ErrorResponse)
}

// Connect returns a platform.Platform implementation that can be used
// to talk to a particular instance.
func (n *NATS) Connect(instID flux.InstanceID) (platform.Platform, error) {
	return &requester{
		conn:     n.snd,
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
	sub, err := n.rcv.ChanSubscribe(string(instID)+".Platform.>", requests)
	if err != nil {
		done <- err
		return
	}

	go func() {
		var err error
		for request := range requests {
			switch {
			case strings.HasSuffix(request.Subject, methodPing):
				var p ping
				err = encoder.Decode(request.Subject, request.Data, &p)
				if err == nil {
					err = remote.Ping()
				}
				n.snd.Publish(request.Reply, PingResponse{makeErrorResponse(err)})
			case strings.HasSuffix(request.Subject, methodAllServices):
				var (
					req fluxrpc.AllServicesRequest
					res []platform.Service
				)
				err = encoder.Decode(request.Subject, request.Data, &req)
				if err == nil {
					res, err = remote.AllServices(req.MaybeNamespace, req.Ignored)
				}
				n.snd.Publish(request.Reply, AllServicesResponse{res, makeErrorResponse(err)})
			case strings.HasSuffix(request.Subject, methodSomeServices):
				var (
					req []flux.ServiceID
					res []platform.Service
				)
				err = encoder.Decode(request.Subject, request.Data, &req)
				if err == nil {
					res, err = remote.SomeServices(req)
				}
				n.snd.Publish(request.Reply, SomeServicesResponse{res, makeErrorResponse(err)})
			case strings.HasSuffix(request.Subject, methodRegrade):
				var (
					req []platform.RegradeSpec
				)
				err = encoder.Decode(request.Subject, request.Data, &req)
				if err == nil {
					err = remote.Regrade(req)
				}
				response := RegradeResponse{}
				switch regradeErr := err.(type) {
				case platform.RegradeError:
					result := fluxrpc.RegradeResult{}
					for s, e := range regradeErr {
						result[s] = e.Error()
					}
					response.Result = result
				default:
					response.ErrorResponse = makeErrorResponse(err)
				}
				n.snd.Publish(request.Reply, response)
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
