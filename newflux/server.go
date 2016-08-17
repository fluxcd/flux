package flux

import "errors"

type server struct{}

var _ Service = &server{}

func (s *server) ListServices() ([]ServiceDescription, error) {
	return nil, errors.New("not implemented")
}

func (s *server) ListImages(ServiceSpec) ([]ImageDescription, error) {
	return nil, errors.New("not implemented")
}

func (s *server) Release(ServiceSpec, ImageSpec) error {
	return errors.New("not implemented")
}

func (s *server) Automate(ServiceID) error {
	return errors.New("not implemented")
}

func (s *server) Deautomate(ServiceID) error {
	return errors.New("not implemented")
}

func (s *server) History(ServiceID) ([]HistoryEntry, error) {
	return nil, errors.New("not implemented")
}
