package main

import (
	"bufio"
	"fmt"
	"net"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
)

type daemonOpts struct {
	listenAddr string
}

func (opts *daemonOpts) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&opts.listenAddr, "listen", "l", ":3030", "the address to listen for flux API clients on")
}

func (opts *daemonOpts) run(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("did not expect any arguments")
	}
	srv, err := net.Listen("tcp", opts.listenAddr)
	if err != nil {
		return err
	}
	log.Infof("listening on %s", opts.listenAddr)
	for {
		conn, err := srv.Accept()
		if err != nil {
			log.Errorf("connection error: %s", err)
		}
		go echo(conn)
	}
}

func echo(conn net.Conn) {
	log.Debugf("open connection from %s", conn.RemoteAddr())
	lines := bufio.NewScanner(conn)
	for lines.Scan() {
		conn.Write([]byte(lines.Text()))
	}
	log.Debugf("close connection from %s", conn.RemoteAddr())
}

func main() {
	opts := &daemonOpts{}
	cmd := &cobra.Command{
		Use:   "fluxd",
		Short: "the flux deployment daemon",
		RunE:  opts.run,
	}
	opts.addFlags(cmd)
	cmd.Execute()
}
