package flux

import "errors"

type server struct{}

func NewServer() Service {
	return &server{}
}

func (s *server) ListServices() ([]ServiceDescription, error) {
	return nil, errors.New("ListServices not implemented by server")
}

func (s *server) ListImages(ServiceSpec) ([]ImageDescription, error) {
	return nil, errors.New("ListImages not implemented by server")
}

func (s *server) Release(ServiceSpec, ImageSpec) error {
	return errors.New("Release not implemented by server")
}

func (s *server) Automate(ServiceID) error {
	return errors.New("Automate not implemented by server")
}

func (s *server) Deautomate(ServiceID) error {
	return errors.New("Deautomate not implemented by server")
}

func (s *server) History(ServiceSpec) ([]HistoryEntry, error) {
	return nil, errors.New("History not implemented by server")
}
