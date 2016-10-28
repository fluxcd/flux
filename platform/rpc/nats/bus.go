package nats

import (
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/platform/rpc"
)

const (
	timeout      = 5 * time.Second
	presenceTick = 10 * time.Millisecond
)

type NATS struct {
	url  string
	conn *nats.EncodedConn
}

var _ platform.MessageBus = &NATS{}

func NewMessageBus(url string) (*NATS, error) {
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	encConn, err := nats.NewEncodedConn(conn, nats.JSON_ENCODER)
	if err != nil {
		return nil, err
	}
	return &NATS{
		url:  url,
		conn: encConn,
	}, nil
}

// Wait up to `timeout` for a particular instance to connect. Mostly
// useful for synchronising during testing.
func (n *NATS) AwaitPresence(instID flux.InstanceID, timeout time.Duration) error {
	timer := time.After(timeout)
	attempts := time.NewTicker(presenceTick)
	defer attempts.Stop()

	var pres Presence
	for {
		select {
		case <-attempts.C:
			if err := n.conn.Request(string(instID)+".Platform.Presence", Presence{}, &pres, presenceTick); err == nil {
				return nil
			}
		case <-timer:
			return errors.New("timeout")
		}
	}
}

type requester struct {
	conn    *nats.EncodedConn
	subject string
}

type Presence struct{}

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

func (r *requester) AllServices(ns string, ig flux.ServiceIDSet) ([]platform.Service, error) {
	var response AllServicesResponse
	if err := r.conn.Request(r.subject+".Platform.AllServices", rpc.AllServicesRequest{ns, ig}, &response, timeout); err != nil {
		return nil, err
	}
	return response.Services, maybeError(response.Error)
}

func (r *requester) SomeServices(incl []flux.ServiceID) ([]platform.Service, error) {
	var response SomeServicesResponse
	if err := r.conn.Request(r.subject+".Platform.SomeServices", incl, &response, timeout); err != nil {
		return nil, err
	}
	return response.Services, maybeError(response.Error)
}

func (r *requester) Regrade(specs []platform.RegradeSpec) error {
	var response RegradeResponse
	if err := r.conn.Request(r.subject+".Platform.Regrade", specs, &response, timeout); err != nil {
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

// Connect returns a platform.Platform implementation that can be used
// to talk to a particular instance.
func (n *NATS) Connect(instID flux.InstanceID) (platform.Platform, error) {
	return &requester{
		conn:    n.conn,
		subject: string(instID),
	}, nil
}

// Subscribe registers a remote platform.Platform implementation as
// representing a particular instance ID, blocking indefinitely.
func (n *NATS) Subscribe(instID flux.InstanceID, remote platform.Platform) error {
	type req struct {
		reply   string
		request interface{}
	}
	requests := make(chan req)
	errc := make(chan error)

	n.conn.Subscribe(string(instID)+".Platform.Presence", func(_, reply string, presReq *Presence) {
		requests <- req{reply, presReq}
	})
	n.conn.Subscribe(string(instID)+".Platform.AllServices", func(subj, reply string, asReq *rpc.AllServicesRequest) {
		requests <- req{reply, asReq}
	})
	n.conn.Subscribe(string(instID)+".Platform.SomeServices", func(subj, reply string, ssReq *[]flux.ServiceID) {
		requests <- req{reply, ssReq}
	})
	n.conn.Subscribe(string(instID)+".Platform.Regrade", func(subj, reply string, rgdReq *[]platform.RegradeSpec) {
		requests <- req{reply, rgdReq}
	})

	for {
		select {
		case r := <-requests:
			switch request := r.request.(type) {
			case *Presence:
				n.conn.Publish(r.reply, Presence{})
			case *rpc.AllServicesRequest:
				ss, err := remote.AllServices(request.MaybeNamespace, request.Ignored)
				n.conn.Publish(r.reply, AllServicesResponse{ss, maybeString(err)})
			case *[]flux.ServiceID:
				ss, err := remote.SomeServices(*request)
				n.conn.Publish(r.reply, SomeServicesResponse{ss, maybeString(err)})
			case *[]platform.RegradeSpec:
				resp := RegradeResponse{}
				err := remote.Regrade(*request)
				switch err := err.(type) {
				case platform.RegradeError:
					result := rpc.RegradeResult{}
					for s, e := range err {
						result[s] = e.Error()
					}
					resp.Result = result
				default:
					resp.Error = maybeString(err)
				}
				n.conn.Publish(r.reply, resp)
			default:
				errc <- errors.New(fmt.Sprintf("Unknown request value %+v", r.request))
			}
		case err := <-errc:
			errc <- err
			close(errc)
			break
		}
	}
	return <-errc
}
