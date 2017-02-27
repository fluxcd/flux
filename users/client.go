package users

import (
	"fmt"

	"github.com/weaveworks/flux"
)

// Client is a grpc client to the users service.
type Client struct{}

func NewClient(addr string) *Client {
	return &Client{}
}

// ExternalInstanceID implements instance.IDMapper
func (c *Client) ExternalInstanceID(internal flux.InstanceID) (string, error) {
	return "", fmt.Errorf("TODO: implement users.Client.ExternalInstanceID")
}

// InternalInstanceID implements instance.IDMapper
func (c *Client) InternalInstanceID(external string) (flux.InstanceID, error) {
	return "", fmt.Errorf("TODO: implement users.Client.InternalInstanceID")
}

func (c *Client) Close() error {
	return nil
}
