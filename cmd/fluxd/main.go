package main

import (
	"bufio"
	"fmt"
	"net"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/spf13/cobra"
)

func main() {
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
		logger = log.NewContext(logger).With("caller", log.DefaultCaller)
	}
	opts := &daemonOpts{
		logger: logger,
	}
	cmd := &cobra.Command{
		Use:   "fluxd",
		Short: "the flux deployment daemon",
		RunE:  opts.run,
	}
	opts.addFlags(cmd)
	cmd.Execute()
}

type daemonOpts struct {
	logger     log.Logger
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
	opts.logger.Log("addr", opts.listenAddr, "msg", "listening")
	for {
		conn, err := srv.Accept()
		if err != nil {
			opts.logger.Log("err", err)
		}
		go echo(conn, opts.logger)
	}
}

func echo(conn net.Conn, logger log.Logger) {
	logger.Log("addr", conn.RemoteAddr(), "msg", "accepted")
	defer logger.Log("addr", conn.RemoteAddr(), "msg", "closed")
	lines := bufio.NewScanner(conn)
	for lines.Scan() {
		conn.Write([]byte(lines.Text() + "\n")) // Scan strips newline
	}
}
