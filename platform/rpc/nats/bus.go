package nats

import (
	"errors"
	"strings"
	"time"

	"github.com/nats-io/nats"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/platform/rpc"
)

const (
	timeout      = 5 * time.Second
	presenceTick = 50 * time.Millisecond
	encoderType  = nats.JSON_ENCODER

	methodPing         = ".Platform.Ping"
	methodAllServices  = ".Platform.AllServices"
	methodSomeServices = ".Platform.SomeServices"
	methodRegrade      = ".Platform.Regrade"
)

type ping struct{}

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
	var response ping
	return n.snd.Request(string(instID)+methodPing, ping{}, &response, timeout)
}

type AllServicesResponse struct {
	Services []platform.Service
	Error    string
}

type SomeServicesResponse struct {
	Services []platform.Service
	Error    string
}

type RegradeResponse struct {
	Result rpc.RegradeResult
	Error  string
}

func maybeError(msg string) error {
	if msg != "" {
		return errors.New(msg)
	}
	return nil
}

func maybeString(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

// requester just collect the things you need to make a request
// together
type requester struct {
	conn     *nats.EncodedConn
	instance string
}

func (r *requester) AllServices(ns string, ig flux.ServiceIDSet) ([]platform.Service, error) {
	var response AllServicesResponse
	if err := r.conn.Request(r.instance+methodAllServices, rpc.AllServicesRequest{ns, ig}, &response, timeout); err != nil {
		return nil, err
	}
	return response.Services, maybeError(response.Error)
}

func (r *requester) SomeServices(incl []flux.ServiceID) ([]platform.Service, error) {
	var response SomeServicesResponse
	if err := r.conn.Request(r.instance+methodSomeServices, incl, &response, timeout); err != nil {
		return nil, err
	}
	return response.Services, maybeError(response.Error)
}

func (r *requester) Regrade(specs []platform.RegradeSpec) error {
	var response RegradeResponse
	if err := r.conn.Request(r.instance+methodRegrade, specs, &response, timeout); err != nil {
		return err
	}
	if len(response.Result) > 0 {
		errs := platform.RegradeError{}
		for s, e := range response.Result {
			errs[s] = errors.New(e)
		}
		return errs
	}
	return maybeError(response.Error)
}

func (r *requester) Ping() error {
	var response ping
	return r.conn.Request(r.instance+methodPing, ping{}, &response, timeout)
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
// representing a particular instance ID, blocking indefinitely.
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
				if err = encoder.Decode(request.Subject, request.Data, &p); err != nil {
					break
				}
				n.snd.Publish(request.Reply, ping{})
			case strings.HasSuffix(request.Subject, methodAllServices):
				var (
					req rpc.AllServicesRequest
					res []platform.Service
				)
				err = encoder.Decode(request.Subject, request.Data, &req)
				if err == nil {
					res, err = remote.AllServices(req.MaybeNamespace, req.Ignored)
				}
				n.snd.Publish(request.Reply, AllServicesResponse{res, maybeString(err)})
			case strings.HasSuffix(request.Subject, methodSomeServices):
				var (
					req []flux.ServiceID
					res []platform.Service
				)
				err = encoder.Decode(request.Subject, request.Data, &req)
				if err == nil {
					res, err = remote.SomeServices(req)
				}
				n.snd.Publish(request.Reply, SomeServicesResponse{res, maybeString(err)})
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
					result := rpc.RegradeResult{}
					for s, e := range regradeErr {
						result[s] = e.Error()
					}
					response.Result = result
				default:
					response.Error = maybeString(err)
				}
				n.snd.Publish(request.Reply, response)
			default:
				err = errors.New("unknown message: " + request.Subject)
			}
			if err != nil {
				sub.Unsubscribe()
				close(requests)
				done <- err
				return
			}
		}
	}()
}
