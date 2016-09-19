package db

import (
	"errors"

	"github.com/weaveworks/fluxy/flux"
	"github.com/weaveworks/fluxy/flux/history"
	"github.com/weaveworks/fluxy/flux/release"
)

// DB provides implementations for several utility interfaces thru a shared
// database, currently Postgres.
type DB struct {
	//
}

// Demux satisfies the orgmap.Demuxer interface.
func (db *DB) Demux(orgID string) (ref string, err error) {
	return "", errors.New("not implemented")
}

// Automate implements automator.Automator.
func (db *DB) Automate(s flux.ServiceID) error {
	return errors.New("not implemented")
}

// Deautomate implements automator.Automator.
func (db *DB) Deautomate(s flux.ServiceID) error {
	return errors.New("not implemented")
}

// PutJob implements release.JobWriter.
func (db *DB) PutJob(s release.JobSpec) (release.ID, error) {
	return "", errors.New("not implemented")
}

// GetJob implements release.JobReader.
func (db *DB) GetJob(id release.ID) (release.Job, error) {
	return release.Job{}, errors.New("not implemented")
}

// NextJob implements release.JobPopper.
func (db *DB) NextJob() (release.Job, error) {
	return release.Job{}, errors.New("not implemented")
}

// UpdateJob implements release.JobPopper.
func (db *DB) UpdateJob(j release.Job) error {
	return errors.New("not implemented")
}

// WriteEvent implements history.EventWriter.
func (db *DB) WriteEvent(e history.Event) error {
	return errors.New("not implemented")
}

// ReadEvents implements history.EventReader.
func (db *DB) ReadEvents(spec flux.ServiceSpec, n int) ([]history.Event, error) {
	return nil, errors.New("not implemented")
}
